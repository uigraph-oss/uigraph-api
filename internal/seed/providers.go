package seed

import (
	"github.com/uigraph/app/internal/identity"
)

// providerTemplates returns the baseline OAuth provider configurations that
// are replicated into every seeded org. OrgID is left zero; the caller fills
// it in per org.
//
// Source: dev cluster's Default-org oauth_provider_config rows
// (auth0, github, google). All three are configured as type=generic with
// explicit endpoints; client_id and client_secret are copied verbatim so the
// dev SSO flow works for the seeded orgs out of the box.
func providerTemplates() []identity.OAuthProviderConfig {
	return []identity.OAuthProviderConfig{
		{
			ProviderName: "auth0",
			Type:         "generic",
			DisplayName:  "Auth0",
			ClientID:     "58FrPzRnWKR6iE34ORgQtB2U8um9gYUX",
			ClientSecret: "IGxfkB2Hdvc-nrhZvG0aW3sqkzS0DbSLLwGI8caOPWqWbzpUjq4x60KzwndsTVcH",
			AuthURL:      "https://nazmussayad.us.auth0.com/authorize",
			TokenURL:     "https://nazmussayad.us.auth0.com/oauth/token",
			UserinfoURL:  "https://nazmussayad.us.auth0.com/userinfo",
			Scopes:       "openid profile email",
			AllowSignUp:  true,
			EmailClaim:   "email",
			NameClaim:    "name",
			SubClaim:     "sub",
		},
		{
			ProviderName: "github",
			Type:         "generic",
			DisplayName:  "GitHub",
			ClientID:     "Ov23liDoaN78Gt6H9cn8",
			ClientSecret: "7645d7fb026fcfdff91e553633c68c6526caad02",
			AuthURL:      "https://github.com/login/oauth/authorize",
			TokenURL:     "https://github.com/login/oauth/access_token",
			UserinfoURL:  "https://api.github.com/user",
			Scopes:       "read:user user:email",
			AllowSignUp:  true,
			EmailClaim:   "email",
			NameClaim:    "name",
			SubClaim:     "sub",
		},
		{
			ProviderName: "google",
			Type:         "generic",
			DisplayName:  "Google",
			ClientID:     "134459428540-gsbiflsop4pqepmfd47qe972482tu39j.apps.googleusercontent.com",
			ClientSecret: "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",
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     "https://oauth2.googleapis.com/token",
			UserinfoURL:  "https://openidconnect.googleapis.com/v1/userinfo",
			Scopes:       "openid profile email",
			AllowSignUp:  true,
			EmailClaim:   "email",
			NameClaim:    "name",
			SubClaim:     "sub",
		},
	}
}
