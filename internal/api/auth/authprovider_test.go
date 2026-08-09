package auth

import (
	"testing"

	"github.com/uigraph/app/internal/identity"
)

func TestSamlProviderForCarriesEveryField(t *testing.T) {
	p := &identity.AuthProvider{
		OrgID:        "org-1",
		Slug:         "acme-okta",
		Kind:         identity.KindSAML,
		IDPEntityID:  "https://idp.test/entity",
		IDPSSOURL:    "https://idp.test/sso",
		IDPCert:      "idp-cert-pem",
		SPCert:       "sp-cert-pem",
		SPKey:        "encrypted-sp-key",
		SignRequests: true,
		NameIDFormat: "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
	}

	got := samlProviderFor(p, "https://app.uigraph.test", "decrypted-sp-key")

	want := map[string]struct{ got, want string }{
		"IDPEntityID":  {got.IDPEntityID, p.IDPEntityID},
		"IDPSSOURL":    {got.IDPSSOURL, p.IDPSSOURL},
		"IDPCert":      {got.IDPCert, p.IDPCert},
		"SPCert":       {got.SPCert, p.SPCert},
		"SPKey":        {got.SPKey, "decrypted-sp-key"},
		"NameIDFormat": {got.NameIDFormat, p.NameIDFormat},
		"SPEntityID":   {got.SPEntityID, samlMetadataURL("https://app.uigraph.test", p.OrgID, p.Slug)},
	}
	for field, v := range want {
		if v.got != v.want {
			t.Errorf("%s = %q, want %q", field, v.got, v.want)
		}
	}

	if !got.SignRequests {
		t.Error("SignRequests should be carried through")
	}
}

func TestSamlProviderForKeepsKeypairWhenNotSigning(t *testing.T) {
	p := &identity.AuthProvider{
		OrgID:        "org-1",
		Slug:         "acme-okta",
		IDPSSOURL:    "https://idp.test/sso",
		IDPCert:      "idp-cert-pem",
		SPCert:       "sp-cert-pem",
		SPKey:        "encrypted-sp-key",
		SignRequests: false,
	}

	got := samlProviderFor(p, "https://app.uigraph.test", "decrypted-sp-key")

	if got.SPCert == "" || got.SPKey == "" {
		t.Fatal("the SP keypair must be carried even when requests are unsigned")
	}
	if got.SignRequests {
		t.Error("SignRequests should stay false")
	}
}
