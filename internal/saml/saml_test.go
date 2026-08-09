package saml

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

const (
	testACSURL      = "https://app.uigraph.test/api/v1/auth/saml/p1/acs"
	testSPEntityID  = "https://app.uigraph.test/saml/p1"
	testIDPEntityID = "https://idp.test/entity"
	testIDPSSOURL   = "https://idp.test/sso"
	testRequestID   = "id-0123456789abcdef"
)

type idp struct {
	key     *rsa.PrivateKey
	cert    *x509.Certificate
	certPEM string
}

func newIDP(t *testing.T) idp {
	t.Helper()
	certPEM, keyPEM, err := GenerateSPKeypair(testIDPEntityID)
	if err != nil {
		t.Fatalf("generate idp keypair: %v", err)
	}
	key, cert, err := keypair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("parse idp keypair: %v", err)
	}
	return idp{key: key, cert: cert, certPEM: certPEM}
}

func (i idp) provider() Provider {
	return Provider{
		IDPEntityID: testIDPEntityID,
		IDPSSOURL:   testIDPSSOURL,
		IDPCert:     i.certPEM,
		SPEntityID:  testSPEntityID,
	}
}

type assertionOpts struct {
	audience        string
	inResponseTo    string
	recipient       string
	notOnOrAfter    time.Time
	tamperAfterSign bool
}

func defaults() assertionOpts {
	return assertionOpts{
		audience:     testSPEntityID,
		inResponseTo: testRequestID,
		recipient:    testACSURL,
		notOnOrAfter: time.Now().Add(5 * time.Minute),
	}
}

// signedResponse builds a SAML Response whose Assertion carries a valid
// enveloped signature from the IdP key, then base64-encodes it for the ACS form.
func (i idp) signedResponse(t *testing.T, o assertionOpts) string {
	t.Helper()
	now := time.Now().UTC()
	stamp := func(tm time.Time) string { return tm.UTC().Format(time.RFC3339) }

	assertionXML := fmt.Sprintf(`<saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="id-assertion-1" IssueInstant="%s" Version="2.0">
  <saml:Issuer>%s</saml:Issuer>
  <saml:Subject>
    <saml:NameID Format="urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress">ada@example.com</saml:NameID>
    <saml:SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer">
      <saml:SubjectConfirmationData InResponseTo="%s" NotOnOrAfter="%s" Recipient="%s"></saml:SubjectConfirmationData>
    </saml:SubjectConfirmation>
  </saml:Subject>
  <saml:Conditions NotBefore="%s" NotOnOrAfter="%s">
    <saml:AudienceRestriction>
      <saml:Audience>%s</saml:Audience>
    </saml:AudienceRestriction>
  </saml:Conditions>
  <saml:AuthnStatement AuthnInstant="%s" SessionIndex="session-1">
    <saml:AuthnContext>
      <saml:AuthnContextClassRef>urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport</saml:AuthnContextClassRef>
    </saml:AuthnContext>
  </saml:AuthnStatement>
  <saml:AttributeStatement>
    <saml:Attribute Name="http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress" FriendlyName="email">
      <saml:AttributeValue>ada@example.com</saml:AttributeValue>
    </saml:Attribute>
    <saml:Attribute Name="groups">
      <saml:AttributeValue>eng</saml:AttributeValue>
      <saml:AttributeValue>platform-admin</saml:AttributeValue>
    </saml:Attribute>
  </saml:AttributeStatement>
</saml:Assertion>`,
		stamp(now), testIDPEntityID,
		o.inResponseTo, stamp(o.notOnOrAfter), o.recipient,
		stamp(now.Add(-time.Minute)), stamp(o.notOnOrAfter),
		o.audience,
		stamp(now),
	)

	doc := etree.NewDocument()
	if err := doc.ReadFromString(assertionXML); err != nil {
		t.Fatalf("parse assertion: %v", err)
	}

	store := dsig.TLSCertKeyStore(tls.Certificate{
		PrivateKey:  i.key,
		Certificate: [][]byte{i.cert.Raw},
	})
	sigCtx := dsig.NewDefaultSigningContext(store)
	// Exclusive canonicalization, as every real IdP uses: the assertion is signed
	// standalone and then embedded in a Response that declares its own prefixes,
	// so the signature must survive a change of surrounding namespace context.
	sigCtx.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	signed, err := sigCtx.SignEnveloped(doc.Root())
	if err != nil {
		t.Fatalf("sign assertion: %v", err)
	}

	if o.tamperAfterSign {
		for _, el := range signed.FindElements("//NameID") {
			el.SetText("mallory@example.com")
		}
	}

	signedDoc := etree.NewDocument()
	signedDoc.SetRoot(signed)
	signedAssertion, err := signedDoc.WriteToString()
	if err != nil {
		t.Fatalf("serialise assertion: %v", err)
	}

	responseXML := fmt.Sprintf(`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" Destination="%s" ID="id-response-1" InResponseTo="%s" IssueInstant="%s" Version="2.0">
  <saml:Issuer>%s</saml:Issuer>
  <samlp:Status><samlp:StatusCode Value="urn:oasis:names:tc:SAML:2.0:status:Success"></samlp:StatusCode></samlp:Status>
  %s
</samlp:Response>`, testACSURL, testRequestID, stamp(now), testIDPEntityID, signedAssertion)

	return base64.StdEncoding.EncodeToString([]byte(responseXML))
}

func acsRequest(t *testing.T, samlResponse string) *http.Request {
	t.Helper()
	form := url.Values{"SAMLResponse": {samlResponse}, "RelayState": {"st4te"}}
	r := httptest.NewRequest(http.MethodPost, testACSURL, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestValidateResponseAcceptsValidAssertion(t *testing.T) {
	i := newIDP(t)
	req := acsRequest(t, i.signedResponse(t, defaults()))

	id, err := ValidateResponse(i.provider(), testACSURL, req, testRequestID)
	if err != nil {
		t.Fatalf("expected a valid assertion to be accepted: %v", err)
	}
	if id.NameID != "ada@example.com" {
		t.Fatalf("NameID = %q", id.NameID)
	}
	if got := id.Attributes["email"]; got != "ada@example.com" {
		t.Fatalf("FriendlyName should be indexed, got %#v", got)
	}
	if got := id.Attributes["http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"]; got != "ada@example.com" {
		t.Fatalf("Name should be indexed, got %#v", got)
	}
	groups, ok := id.Attributes["groups"].([]string)
	if !ok || len(groups) != 2 || groups[0] != "eng" || groups[1] != "platform-admin" {
		t.Fatalf("multi-valued attribute should stay a slice, got %#v", id.Attributes["groups"])
	}
}

func TestValidateResponseRejectsTamperedSignature(t *testing.T) {
	i := newIDP(t)
	o := defaults()
	o.tamperAfterSign = true
	req := acsRequest(t, i.signedResponse(t, o))

	if _, err := ValidateResponse(i.provider(), testACSURL, req, testRequestID); err == nil {
		t.Fatal("an assertion modified after signing must be rejected")
	}
}

func TestValidateResponseRejectsUnsignedAssertion(t *testing.T) {
	i := newIDP(t)
	encoded := i.signedResponse(t, defaults())
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(raw); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	for _, el := range doc.FindElements("//Signature") {
		el.Parent().RemoveChild(el)
	}
	stripped, err := doc.WriteToString()
	if err != nil {
		t.Fatalf("serialise: %v", err)
	}
	req := acsRequest(t, base64.StdEncoding.EncodeToString([]byte(stripped)))

	if _, err := ValidateResponse(i.provider(), testACSURL, req, testRequestID); err == nil {
		t.Fatal("an unsigned assertion must be rejected")
	}
}

func TestValidateResponseRejectsWrongAudience(t *testing.T) {
	i := newIDP(t)
	o := defaults()
	o.audience = "https://someone-else.test/saml"
	req := acsRequest(t, i.signedResponse(t, o))

	if _, err := ValidateResponse(i.provider(), testACSURL, req, testRequestID); err == nil {
		t.Fatal("an assertion addressed to another audience must be rejected")
	}
}

func TestValidateResponseRejectsExpiredAssertion(t *testing.T) {
	i := newIDP(t)
	o := defaults()
	o.notOnOrAfter = time.Now().Add(-24 * time.Hour)
	req := acsRequest(t, i.signedResponse(t, o))

	if _, err := ValidateResponse(i.provider(), testACSURL, req, testRequestID); err == nil {
		t.Fatal("an expired assertion must be rejected")
	}
}

func TestValidateResponseRejectsMismatchedRequestID(t *testing.T) {
	i := newIDP(t)
	req := acsRequest(t, i.signedResponse(t, defaults()))

	if _, err := ValidateResponse(i.provider(), testACSURL, req, "id-some-other-request"); err == nil {
		t.Fatal("an assertion answering a different request must be rejected")
	}
}

func TestValidateResponseRequiresAPendingRequest(t *testing.T) {
	i := newIDP(t)
	req := acsRequest(t, i.signedResponse(t, defaults()))

	if _, err := ValidateResponse(i.provider(), testACSURL, req, ""); err == nil {
		t.Fatal("a response with no pending authn request must be rejected")
	}
}

func TestAuthnRequestRedirect(t *testing.T) {
	i := newIDP(t)

	redirect, requestID, err := AuthnRequest(i.provider(), testACSURL, "st4te")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestID == "" {
		t.Fatal("a request ID must be returned so the callback can check InResponseTo")
	}
	u, err := url.Parse(redirect)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	if u.Host != "idp.test" || u.Path != "/sso" {
		t.Fatalf("redirect should target the IdP SSO URL, got %q", redirect)
	}
	if u.Query().Get("SAMLRequest") == "" {
		t.Fatal("redirect is missing SAMLRequest")
	}
	if u.Query().Get("RelayState") != "st4te" {
		t.Fatalf("RelayState = %q", u.Query().Get("RelayState"))
	}
	if u.Query().Get("Signature") != "" {
		t.Fatal("an unsigned provider must not send a Signature")
	}
}

func TestAuthnRequestSigned(t *testing.T) {
	i := newIDP(t)
	certPEM, keyPEM, err := GenerateSPKeypair(testSPEntityID)
	if err != nil {
		t.Fatalf("generate sp keypair: %v", err)
	}
	p := i.provider()
	p.SignRequests = true
	p.SPCert = certPEM
	p.SPKey = keyPEM

	redirect, _, err := AuthnRequest(p, testACSURL, "st4te")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u, err := url.Parse(redirect)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	if u.Query().Get("Signature") == "" || u.Query().Get("SigAlg") == "" {
		t.Fatal("a signing provider must send SigAlg and Signature")
	}
}

func TestSPMetadata(t *testing.T) {
	i := newIDP(t)

	out, err := SPMetadata(i.provider(), testACSURL, "https://app.uigraph.test/saml/p1/metadata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, testSPEntityID) {
		t.Fatal("metadata should carry the SP entity ID")
	}
	if !strings.Contains(out, testACSURL) {
		t.Fatal("metadata should carry the ACS URL")
	}
}

func TestNormalizeCerts(t *testing.T) {
	firstPEM, _, err := GenerateSPKeypair("first")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	secondPEM, _, err := GenerateSPKeypair("second")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	firstBlock, _ := pem.Decode([]byte(firstPEM))
	secondBlock, _ := pem.Decode([]byte(secondPEM))
	firstBare := base64.StdEncoding.EncodeToString(firstBlock.Bytes)
	secondBare := base64.StdEncoding.EncodeToString(secondBlock.Bytes)

	fromPEM, err := normalizeCerts(firstPEM)
	if err != nil {
		t.Fatalf("PEM should be accepted: %v", err)
	}
	fromBare, err := normalizeCerts(firstBare)
	if err != nil {
		t.Fatalf("bare base64 should be accepted: %v", err)
	}
	if len(fromPEM) != 1 || fromPEM[0] != firstBare {
		t.Fatalf("PEM normalised to %v", fromPEM)
	}
	if len(fromBare) != 1 || fromBare[0] != firstBare {
		t.Fatalf("bare base64 normalised to %v", fromBare)
	}

	both, err := normalizeCerts(firstPEM + secondPEM)
	if err != nil {
		t.Fatalf("concatenated PEM should be accepted: %v", err)
	}
	if len(both) != 2 || both[0] != firstBare || both[1] != secondBare {
		t.Fatalf("concatenated PEM normalised to %d certs, want both in order", len(both))
	}

	if _, err := normalizeCerts("   "); err == nil {
		t.Fatal("an empty certificate must be rejected")
	}
	if _, err := normalizeCerts("this is not a certificate!!"); err == nil {
		t.Fatal("a non-base64 certificate must be rejected")
	}
}

// An IdP mid-rollover publishes several signing certificates and may sign with
// any of them, so every certificate on the provider has to be trusted, not just
// the first.
func TestValidateResponseAcceptsAnyConfiguredCert(t *testing.T) {
	retiring := newIDP(t)
	active := newIDP(t)

	p := active.provider()
	p.IDPCert = retiring.certPEM + active.certPEM

	id, err := ValidateResponse(p, testACSURL, acsRequest(t, active.signedResponse(t, defaults())), testRequestID)
	if err != nil {
		t.Fatalf("a response signed by the second configured cert should validate: %v", err)
	}
	if id.NameID != "ada@example.com" {
		t.Fatalf("NameID = %q", id.NameID)
	}

	stranger := newIDP(t)
	if _, err := ValidateResponse(p, testACSURL, acsRequest(t, stranger.signedResponse(t, defaults())), testRequestID); err == nil {
		t.Fatal("a response signed by an unconfigured cert must be rejected")
	}
}

func TestParseMetadataRequiresASource(t *testing.T) {
	if _, err := ParseMetadata(t.Context(), "", ""); err == nil {
		t.Fatal("expected an error when neither a URL nor a document is given")
	}
}

func TestParseMetadataDocument(t *testing.T) {
	i := newIDP(t)
	block, _ := pem.Decode([]byte(i.certPEM))
	bare := base64.StdEncoding.EncodeToString(block.Bytes)

	doc := fmt.Sprintf(`<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="%s">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing">
      <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><X509Data><X509Certificate>%s</X509Certificate></X509Data></KeyInfo>
    </KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="%s"/>
  </IDPSSODescriptor>
</EntityDescriptor>`, testIDPEntityID, bare, testIDPSSOURL)

	md, err := ParseMetadata(t.Context(), "", doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if md.EntityID != testIDPEntityID {
		t.Fatalf("EntityID = %q", md.EntityID)
	}
	if md.SSOURL != testIDPSSOURL {
		t.Fatalf("SSOURL = %q", md.SSOURL)
	}
	certs, err := normalizeCerts(md.Cert)
	if err != nil {
		t.Fatalf("extracted certificate should normalise: %v", err)
	}
	if len(certs) != 1 || certs[0] != bare {
		t.Fatal("the signing certificate should be extracted")
	}
}

func TestParseMetadataKeepsEverySigningCert(t *testing.T) {
	first := newIDP(t)
	second := newIDP(t)
	firstBlock, _ := pem.Decode([]byte(first.certPEM))
	secondBlock, _ := pem.Decode([]byte(second.certPEM))
	firstBare := base64.StdEncoding.EncodeToString(firstBlock.Bytes)
	secondBare := base64.StdEncoding.EncodeToString(secondBlock.Bytes)

	doc := fmt.Sprintf(`<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="%s">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing">
      <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><X509Data><X509Certificate>%s</X509Certificate></X509Data></KeyInfo>
    </KeyDescriptor>
    <KeyDescriptor use="signing">
      <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><X509Data><X509Certificate>%s</X509Certificate></X509Data></KeyInfo>
    </KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="%s"/>
  </IDPSSODescriptor>
</EntityDescriptor>`, testIDPEntityID, firstBare, secondBare, testIDPSSOURL)

	md, err := ParseMetadata(t.Context(), "", doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	certs, err := normalizeCerts(md.Cert)
	if err != nil {
		t.Fatalf("extracted certificates should normalise: %v", err)
	}
	if len(certs) != 2 || certs[0] != firstBare || certs[1] != secondBare {
		t.Fatalf("got %d certificates, want both signing certs in document order", len(certs))
	}
}
