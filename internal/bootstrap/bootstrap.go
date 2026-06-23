// Package bootstrap seeds the database with a default org and server-admin user
// on first boot. Run is idempotent: it exits immediately if any user already exists.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/uigraph/app/internal/config"
	"github.com/uigraph/app/internal/org"
)

// Store is the narrow interface bootstrap needs.
type Store interface {
	AnyUserExists(ctx context.Context) (bool, error)
	UpsertUser(ctx context.Context, u org.User) error
	CreateOrg(ctx context.Context, o org.Org) error
	AddMember(ctx context.Context, m org.OrgMember) error
}

// Run seeds the default org and server-admin user if the database has no users.
// Skipped entirely when users already exist, matching Grafana's first-boot pattern.
func Run(ctx context.Context, s Store, cfg *config.Config) error {
	exists, err := s.AnyUserExists(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap: check users: %w", err)
	}
	if exists {
		return nil
	}

	slog.InfoContext(ctx, "bootstrapping default org and admin user")

	now := time.Now().UTC()

	orgID := uuid.NewString()
	if err := s.CreateOrg(ctx, org.Org{
		ID:        orgID,
		Name:      "Main Org",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("bootstrap: create default org: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("bootstrap: hash password: %w", err)
	}

	userID := uuid.NewString()
	login := strings.SplitN(cfg.AdminEmail, "@", 2)[0]
	if err := s.UpsertUser(ctx, org.User{
		ID:                 userID,
		Email:              cfg.AdminEmail,
		Name:               "Admin",
		Login:              login,
		PasswordHash:       string(hash),
		MustChangePassword: cfg.AdminPassword == "admin",
		Role:               "server_admin",
		CreatedAt:          now,
		UpdatedAt:          now,
	}); err != nil {
		return fmt.Errorf("bootstrap: create admin user: %w", err)
	}

	if err := s.AddMember(ctx, org.OrgMember{
		UserID:    userID,
		OrgID:     orgID,
		Role:      "admin",
		Source:    "manual",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("bootstrap: add admin to org: %w", err)
	}

	slog.InfoContext(ctx, "bootstrap complete", "admin", cfg.AdminEmail, "org", "Main Org")
	return nil
}
