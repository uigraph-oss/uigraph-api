// Package bootstrap seeds the database with a server-admin user on first boot.
// Run is idempotent: it exits immediately if any user already exists.
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
	org.UserStore
}

// Run seeds the server-admin user if the database is empty of users.
func Run(ctx context.Context, s Store, cfg *config.Config) error {
	exists, err := s.AnyUserExists(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap: check users: %w", err)
	}
	if exists {
		return nil
	}

	slog.InfoContext(ctx, "bootstrapping server-admin user")

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("bootstrap: hash password: %w", err)
	}

	now := time.Now().UTC()

	login := strings.SplitN(cfg.AdminEmail, "@", 2)[0]
	adminUser := org.User{
		ID:                 uuid.NewString(),
		Email:              cfg.AdminEmail,
		Name:               "Admin",
		Login:              login,
		PasswordHash:       string(hash),
		MustChangePassword: cfg.AdminPassword == "admin",
		Role:               "server_admin",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.UpsertUser(ctx, adminUser); err != nil {
		return fmt.Errorf("bootstrap: create admin user: %w", err)
	}

	slog.InfoContext(ctx, "bootstrap complete", "admin", adminUser.Email)
	return nil
}
