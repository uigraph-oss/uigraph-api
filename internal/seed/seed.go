// Package seed populates the database with a small set of demo organisations,
// a shared set of admin users, and a baseline set of global OAuth providers.
//
// The current seed configuration creates the "acme", "globex", and "initech"
// orgs and two users who are server admins and admin members of every org.
// OAuth providers are seeded once globally (not per org).
//
// Run is idempotent: re-running it against a populated database is a no-op
// for any org/user/provider that already exists.
package seed

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/org"
)

// Store is the narrow persistence interface the seed needs.
// The postgres DB satisfies it via composition.
type Store interface {
	org.OrgStore
	org.UserStore
	authz.RBACStore
	identity.ProviderStore
}

// orgSpec describes one organisation to seed.
type orgSpec struct {
	Slug string
	Name string
}

// orgs is the seed manifest. Extend this slice to add more demo tenants.
var orgs = []orgSpec{
	{Slug: "acme", Name: "Acme Inc"},
	{Slug: "globex", Name: "Globex Corp"},
	{Slug: "initech", Name: "Initech"},
}

// userSpec describes one admin user to seed. Every seeded user is a
// server admin and an admin member of every seeded org.
type userSpec struct {
	Email    string
	Name     string
	Password string
}

// users is the seed manifest of admins.
var users = []userSpec{
	{Email: "test@sayad.cc", Name: "Test", Password: "M1tp^behAg1n%KVx"},
	{Email: "admin@uigraph.app", Name: "Admin", Password: "admin"},
}

// Run seeds the orgs, global providers, and admin users. Safe to call repeatedly.
func Run(ctx context.Context, s Store) error {
	now := time.Now().UTC()

	orgsCreated := 0
	usersCreated := 0
	providersUpserted := 0

	// Create all orgs first.
	createdOrgs := make([]*org.Org, 0, len(orgs))
	for _, spec := range orgs {
		o, isNew, err := ensureOrg(ctx, s, spec, now)
		if err != nil {
			return fmt.Errorf("seed: org %q: %w", spec.Slug, err)
		}
		if isNew {
			orgsCreated++
		}
		createdOrgs = append(createdOrgs, o)
	}

	// Seed global providers once.
	for _, tpl := range providerTemplates() {
		if err := s.UpsertOAuthProvider(ctx, tpl); err != nil {
			return fmt.Errorf("seed: upsert provider %q: %w", tpl.ProviderName, err)
		}
		providersUpserted++
	}

	// Create each user as server admin and grant them admin role in every org.
	for _, spec := range users {
		u, isNew, err := ensureUser(ctx, s, spec, now)
		if err != nil {
			return fmt.Errorf("seed: user %q: %w", spec.Email, err)
		}
		if isNew {
			usersCreated++
		}

		for _, o := range createdOrgs {
			if err := s.UpsertOrgMember(ctx, u.ID, o.ID, authz.RoleAdmin, "seed"); err != nil {
				return fmt.Errorf("seed: assign admin role for %q in %q: %w", spec.Email, o.Slug, err)
			}
		}
	}

	slog.InfoContext(ctx, "seed complete",
		"orgs_created", orgsCreated,
		"users_created", usersCreated,
		"providers_upserted", providersUpserted,
	)
	return nil
}

func ensureOrg(ctx context.Context, s Store, spec orgSpec, now time.Time) (*org.Org, bool, error) {
	existing, err := s.GetOrgBySlug(ctx, spec.Slug)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		slog.InfoContext(ctx, "seed: org exists, reusing", "slug", spec.Slug, "id", existing.ID)
		return existing, false, nil
	}

	o := &org.Org{
		ID:        uuid.NewString(),
		Name:      spec.Name,
		Slug:      spec.Slug,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateOrg(ctx, *o); err != nil {
		return nil, false, err
	}
	slog.InfoContext(ctx, "seed: created org", "slug", spec.Slug, "id", o.ID)
	return o, true, nil
}

func ensureUser(ctx context.Context, s Store, spec userSpec, now time.Time) (*org.User, bool, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(spec.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, false, fmt.Errorf("hash password: %w", err)
	}

	u := org.User{
		ID:                 uuid.NewString(),
		Email:              spec.Email,
		Name:               spec.Name,
		Login:              spec.Email,
		PasswordHash:       string(hash),
		MustChangePassword: false,
		Role:               "server_admin",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.UpsertUser(ctx, u); err != nil {
		return nil, false, err
	}

	// UpsertUser does not return the persisted row; look it up so the caller
	// gets the canonical id (the row's id is stable on conflict since email
	// is the unique key and we generate a fresh one only on insert).
	persisted, err := s.GetUserByEmail(ctx, spec.Email)
	if err != nil {
		return nil, false, err
	}
	if persisted == nil {
		return nil, false, fmt.Errorf("user %q not found after upsert", spec.Email)
	}
	if persisted.ID == u.ID {
		slog.InfoContext(ctx, "seed: created user", "email", spec.Email)
		return persisted, true, nil
	}
	slog.InfoContext(ctx, "seed: user exists, reusing", "email", spec.Email, "id", persisted.ID)
	return persisted, false, nil
}
