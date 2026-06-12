package identity

import "time"

// Session represents an authenticated user session.
type Session struct {
	ID             string
	UserID         string
	TokenHash      string
	PrevTokenHash  string
	AuthTokenSeen  bool
	SeenAt         *time.Time
	UserAgent      string
	ClientIP       string
	RotatedAt      time.Time
	CreatedAt      time.Time
	ExpiresAt      time.Time
	LastActiveAt   time.Time
	SAMLSessionIdx string
	SAMLNameIDHash string
	AuthProvider   string // 'password' or the OAuth provider instance name
}
