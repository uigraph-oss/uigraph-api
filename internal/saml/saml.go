// Package saml wraps github.com/crewjam/saml with the SP-initiated SSO flow
// UIGraph needs, so handlers never touch SAML XML directly.
//
// Only SP-initiated login is supported. IdP-initiated login would require
// accepting an assertion with no InResponseTo, which is exactly the check that
// makes assertions single-use here: the request ID lives in auth_login_state,
// and ConsumeLoginState deletes it atomically on the first callback, so a
// replayed assertion finds no matching request ID. Single Logout is out of
// scope.
package saml

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	csaml "github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	dsig "github.com/russellhaering/goxmldsig"
)

// Provider is the resolved SAML configuration for one org's provider. It mirrors
// the SAML half of identity.AuthProvider, with SPKey already decrypted.
type Provider struct {
	IDPEntityID  string
	IDPSSOURL    string
	IDPCert      string
	SPEntityID   string
	SPCert       string
	SPKey        string
	SignRequests bool
	NameIDFormat string
}

// Metadata is the subset of an IdP's metadata document that a provider stores.
type Metadata struct {
	EntityID string
	SSOURL   string
	Cert     string
}

// Identity is what a validated assertion yields.
//
// Attributes carries every assertion attribute keyed by both its Name and its
// FriendlyName, because IdPs vary in which they populate and role mapping rules
// are written against whichever the admin can see. Single-valued attributes are
// strings, multi-valued ones []string, matching what rolemap expects.
type Identity struct {
	NameID     string
	Attributes map[string]any
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

// ParseMetadata resolves an IdP's entity ID, redirect-binding SSO URL and
// signing certificate from a metadata URL or a pasted metadata document. It runs
// when a provider is saved so the login path never parses XML.
func ParseMetadata(ctx context.Context, metadataURL, metadataXML string) (Metadata, error) {
	var ed *csaml.EntityDescriptor
	var err error

	if strings.TrimSpace(metadataXML) != "" {
		ed, err = samlsp.ParseMetadata([]byte(metadataXML))
		if err != nil {
			return Metadata{}, fmt.Errorf("saml: parse metadata document: %w", err)
		}
	} else if strings.TrimSpace(metadataURL) != "" {
		u, perr := url.Parse(metadataURL)
		if perr != nil {
			return Metadata{}, fmt.Errorf("saml: parse metadata url: %w", perr)
		}
		ed, err = samlsp.FetchMetadata(ctx, httpClient, *u)
		if err != nil {
			return Metadata{}, fmt.Errorf("saml: fetch metadata: %w", err)
		}
	} else {
		return Metadata{}, fmt.Errorf("saml: a metadata url or document is required")
	}

	out := Metadata{EntityID: ed.EntityID}
	for _, idp := range ed.IDPSSODescriptors {
		for _, sso := range idp.SingleSignOnServices {
			if sso.Binding == csaml.HTTPRedirectBinding && out.SSOURL == "" {
				out.SSOURL = sso.Location
			}
		}
		for _, kd := range idp.KeyDescriptors {
			if kd.Use != "" && kd.Use != "signing" {
				continue
			}
			for _, c := range kd.KeyInfo.X509Data.X509Certificates {
				if out.Cert == "" {
					out.Cert = strings.Join(strings.Fields(c.Data), "")
				}
			}
		}
	}

	if out.SSOURL == "" {
		return Metadata{}, fmt.Errorf("saml: metadata has no HTTP-Redirect SSO endpoint")
	}
	if out.Cert == "" {
		return Metadata{}, fmt.Errorf("saml: metadata has no signing certificate")
	}
	return out, nil
}

// GenerateSPKeypair returns a self-signed certificate and private key in PEM
// form, used to sign AuthnRequests. Generated when a SAML provider is created;
// the key is encrypted before it is stored.
func GenerateSPKeypair(entityID string) (certPEM, keyPEM string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("saml: generate sp key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("saml: generate serial: %w", err)
	}

	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: entityID, Organization: []string{"UIGraph"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", fmt.Errorf("saml: create sp certificate: %w", err)
	}

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	return certPEM, keyPEM, nil
}

// AuthnRequest builds the HTTP-Redirect binding URL to send the browser to, and
// returns the request ID to persist so the callback can check InResponseTo.
func AuthnRequest(p Provider, acsURL, relayState string) (redirectURL, requestID string, err error) {
	sp, err := serviceProvider(p, acsURL, "")
	if err != nil {
		return "", "", err
	}

	req, err := sp.MakeAuthenticationRequest(p.IDPSSOURL, csaml.HTTPRedirectBinding, csaml.HTTPPostBinding)
	if err != nil {
		return "", "", fmt.Errorf("saml: build authn request: %w", err)
	}

	u, err := req.Redirect(relayState, sp)
	if err != nil {
		return "", "", fmt.Errorf("saml: sign authn request: %w", err)
	}
	return u.String(), req.ID, nil
}

// ValidateResponse verifies the assertion in an ACS POST and extracts the
// identity. It checks the IdP's signature, the NotBefore / NotOnOrAfter
// conditions, the Audience against the SP entity ID, the Destination against the
// ACS URL, and InResponseTo against expectedRequestID.
func ValidateResponse(p Provider, acsURL string, r *http.Request, expectedRequestID string) (*Identity, error) {
	if expectedRequestID == "" {
		return nil, fmt.Errorf("saml: no pending authn request for this response")
	}

	sp, err := serviceProvider(p, acsURL, "")
	if err != nil {
		return nil, err
	}

	// ParseResponse reads SAMLResponse straight out of r.PostForm without
	// populating it, so the form has to be parsed first.
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("saml: parse acs form: %w", err)
	}

	assertion, err := sp.ParseResponse(r, []string{expectedRequestID})
	if err != nil {
		// crewjam collapses every validation failure into "Authentication
		// failed" and hides the reason in PrivateErr. Unwrap it: without the
		// real reason a misconfigured IdP is undiagnosable from the logs.
		var invalid *csaml.InvalidResponseError
		if errors.As(err, &invalid) && invalid.PrivateErr != nil {
			return nil, fmt.Errorf("saml: validate response: %w", invalid.PrivateErr)
		}
		return nil, fmt.Errorf("saml: validate response: %w", err)
	}

	out := Identity{Attributes: map[string]any{}}
	if assertion.Subject != nil && assertion.Subject.NameID != nil {
		out.NameID = assertion.Subject.NameID.Value
	}
	if out.NameID == "" {
		return nil, fmt.Errorf("saml: assertion has no NameID")
	}

	for _, stmt := range assertion.AttributeStatements {
		for _, attr := range stmt.Attributes {
			values := make([]string, 0, len(attr.Values))
			for _, v := range attr.Values {
				values = append(values, v.Value)
			}
			if attr.Name != "" {
				out.Attributes[attr.Name] = collapse(values)
			}
			if attr.FriendlyName != "" {
				out.Attributes[attr.FriendlyName] = collapse(values)
			}
		}
	}
	return &out, nil
}

// SPMetadata renders the SP metadata document an admin hands to their IdP.
func SPMetadata(p Provider, acsURL, metadataURL string) (string, error) {
	sp, err := serviceProvider(p, acsURL, metadataURL)
	if err != nil {
		return "", err
	}
	out, err := xml.MarshalIndent(sp.Metadata(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("saml: marshal sp metadata: %w", err)
	}
	return xml.Header + string(out), nil
}

// serviceProvider assembles a crewjam ServiceProvider from stored config.
//
// The IdP is described by an entity descriptor synthesised from idp_sso_url and
// idp_cert rather than by the raw metadata document: the fields are normalised
// on save, and an admin may configure a provider by pasting a URL and
// certificate with no metadata document at all.
func serviceProvider(p Provider, acsURL, metadataURL string) (*csaml.ServiceProvider, error) {
	acs, err := url.Parse(acsURL)
	if err != nil {
		return nil, fmt.Errorf("saml: parse acs url: %w", err)
	}

	cert, err := normalizeCert(p.IDPCert)
	if err != nil {
		return nil, err
	}

	sp := csaml.ServiceProvider{
		EntityID:          p.SPEntityID,
		AcsURL:            *acs,
		AuthnNameIDFormat: csaml.NameIDFormat(p.NameIDFormat),
		IDPMetadata: &csaml.EntityDescriptor{
			EntityID: p.IDPEntityID,
			IDPSSODescriptors: []csaml.IDPSSODescriptor{{
				SSODescriptor: csaml.SSODescriptor{
					RoleDescriptor: csaml.RoleDescriptor{
						KeyDescriptors: []csaml.KeyDescriptor{{
							Use: "signing",
							KeyInfo: csaml.KeyInfo{
								X509Data: csaml.X509Data{
									X509Certificates: []csaml.X509Certificate{{Data: cert}},
								},
							},
						}},
					},
				},
				SingleSignOnServices: []csaml.Endpoint{{
					Binding:  csaml.HTTPRedirectBinding,
					Location: p.IDPSSOURL,
				}},
			}},
		},
	}

	if metadataURL != "" {
		md, err := url.Parse(metadataURL)
		if err != nil {
			return nil, fmt.Errorf("saml: parse metadata url: %w", err)
		}
		sp.MetadataURL = *md
	}

	if p.SignRequests {
		key, cert, err := keypair(p.SPCert, p.SPKey)
		if err != nil {
			return nil, err
		}
		sp.Key = key
		sp.Certificate = cert
		sp.SignatureMethod = dsig.RSASHA256SignatureMethod
	}

	return &sp, nil
}

func keypair(certPEM, keyPEM string) (*rsa.PrivateKey, *x509.Certificate, error) {
	keyBlock, _ := pem.Decode([]byte(keyPEM))
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("saml: sp key is not valid PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		parsed, perr := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if perr != nil {
			return nil, nil, fmt.Errorf("saml: parse sp key: %w", err)
		}
		rsaKey, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, fmt.Errorf("saml: sp key is not an RSA key")
		}
		key = rsaKey
	}

	certBlock, _ := pem.Decode([]byte(certPEM))
	if certBlock == nil {
		return nil, nil, fmt.Errorf("saml: sp certificate is not valid PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("saml: parse sp certificate: %w", err)
	}
	return key, cert, nil
}

// normalizeCert accepts either a PEM block or bare base64 and returns the bare
// base64 body that an X509Certificate element expects. IdP consoles hand out
// both forms, and admins paste whichever they were given.
func normalizeCert(cert string) (string, error) {
	cert = strings.TrimSpace(cert)
	if cert == "" {
		return "", fmt.Errorf("saml: idp certificate is required")
	}
	if block, _ := pem.Decode([]byte(cert)); block != nil {
		return base64.StdEncoding.EncodeToString(block.Bytes), nil
	}
	body := strings.Join(strings.Fields(cert), "")
	if _, err := base64.StdEncoding.DecodeString(body); err != nil {
		return "", fmt.Errorf("saml: idp certificate is neither PEM nor base64: %w", err)
	}
	return body, nil
}

// collapse keeps single-valued attributes as plain strings so that an equals
// rule on a scalar attribute reads naturally, while preserving multi-valued
// attributes such as groups.
func collapse(values []string) any {
	if len(values) == 1 {
		return values[0]
	}
	return values
}
