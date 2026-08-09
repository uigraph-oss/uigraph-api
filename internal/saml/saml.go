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

type Metadata struct {
	EntityID string
	SSOURL   string
	Cert     string
}

type Identity struct {
	NameID     string
	Attributes map[string]any
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

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
	var certs []string
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
				der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(c.Data), ""))
				if err != nil {
					return Metadata{}, fmt.Errorf("saml: metadata certificate is not base64: %w", err)
				}
				certs = append(certs, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
			}
		}
	}
	out.Cert = strings.Join(certs, "")

	if out.SSOURL == "" {
		return Metadata{}, fmt.Errorf("saml: metadata has no HTTP-Redirect SSO endpoint")
	}
	if out.Cert == "" {
		return Metadata{}, fmt.Errorf("saml: metadata has no signing certificate")
	}
	return out, nil
}

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

func ValidateResponse(p Provider, acsURL string, r *http.Request, expectedRequestID string) (*Identity, error) {
	if expectedRequestID == "" {
		return nil, fmt.Errorf("saml: no pending authn request for this response")
	}

	sp, err := serviceProvider(p, acsURL, "")
	if err != nil {
		return nil, err
	}

	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("saml: parse acs form: %w", err)
	}

	assertion, err := sp.ParseResponse(r, []string{expectedRequestID})
	if err != nil {
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

func serviceProvider(p Provider, acsURL, metadataURL string) (*csaml.ServiceProvider, error) {
	acs, err := url.Parse(acsURL)
	if err != nil {
		return nil, fmt.Errorf("saml: parse acs url: %w", err)
	}

	certs, err := normalizeCerts(p.IDPCert)
	if err != nil {
		return nil, err
	}
	x509Certs := make([]csaml.X509Certificate, len(certs))
	for i, c := range certs {
		x509Certs[i] = csaml.X509Certificate{Data: c}
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
									X509Certificates: x509Certs,
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

func normalizeCerts(cert string) ([]string, error) {
	cert = strings.TrimSpace(cert)
	if cert == "" {
		return nil, fmt.Errorf("saml: idp certificate is required")
	}

	var out []string
	for rest := []byte(cert); ; {
		block, remainder := pem.Decode(rest)
		if block == nil {
			break
		}
		out = append(out, base64.StdEncoding.EncodeToString(block.Bytes))
		rest = remainder
	}
	if len(out) > 0 {
		return out, nil
	}

	body := strings.Join(strings.Fields(cert), "")
	if _, err := base64.StdEncoding.DecodeString(body); err != nil {
		return nil, fmt.Errorf("saml: idp certificate is neither PEM nor base64: %w", err)
	}
	return []string{body}, nil
}

func collapse(values []string) any {
	if len(values) == 1 {
		return values[0]
	}
	return values
}
