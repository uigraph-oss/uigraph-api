// Package actor resolves a bare actor UUID (as stored in created_by / updated_by /
// deleted_by columns) into public identity info. An ID may belong to either the
// users table or the service_accounts table — they share one UUID space and have no
// foreign keys — so resolution queries users first (user wins on the impossible
// collision case), then service_accounts, and caches the result in Redis.
package actor

import (
	"context"
	"encoding/json"
	"time"

	"github.com/uigraph/app/internal/asset"
	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/store"
)

// positiveTTL is how long a resolved actor is cached.
// negativeTTL is how long an unresolved id is cached so orphaned ids
// (e.g. hard-deleted records) don't re-query both tables every request.
const (
	positiveTTL = 24 * time.Hour
	negativeTTL = 5 * time.Minute
)

// nullSentinel is the cached value for an id that resolves to neither a user
// nor a service account.
const nullSentinel = "null"

// Kind is the kind of principal an actor id resolved to.
type Kind string

const (
	KindUser           Kind = "user"
	KindServiceAccount Kind = "service_account"
)

// Actor is the public identity info for an actor id.
type Actor struct {
	ID       string `json:"id"`
	Type     Kind   `json:"type"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Disabled bool   `json:"disabled"`
	// AvatarAssetID is the stable storage asset id of the avatar; it is cached.
	AvatarAssetID string `json:"avatarAssetId,omitempty"`
	// AvatarURL is a freshly presigned URL minted per response. It is never
	// cached on the actor (the actor cache outlives a presigned URL), so it is
	// always filled after the actor cache read/write via the asset resolver.
	AvatarURL string `json:"avatarUrl,omitempty"`
}

// Resolver resolves actor ids against the store, cached in Redis.
type Resolver struct {
	store  store.Store
	cache  cache.Client    // may be nil
	assets *asset.Resolver // presigns avatar URLs; may be nil
}

func New(s store.Store, c cache.Client, assets *asset.Resolver) *Resolver {
	return &Resolver{store: s, cache: c, assets: assets}
}

// Resolve returns the actor for id, or (nil, nil) if id matches no user or
// service account. The presigned AvatarURL is filled here, outside the actor
// cache, so it never goes stale relative to its presign expiry.
func (r *Resolver) Resolve(ctx context.Context, id string) (*Actor, error) {
	a, err := r.resolveCached(ctx, id)
	if err != nil {
		return nil, err
	}
	if a != nil && a.AvatarAssetID != "" && r.assets != nil {
		if u, uErr := r.assets.Resolve(ctx, a.AvatarAssetID); uErr == nil {
			a.AvatarURL = u
		}
	}
	return a, nil
}

// resolveCached returns the bare actor (no AvatarURL) from cache or store.
func (r *Resolver) resolveCached(ctx context.Context, id string) (*Actor, error) {
	if r.cache != nil {
		v, err := r.cache.Get(ctx, cache.ActorKey(id))
		if err == nil && v == nullSentinel {
			return nil, nil
		}
		if err == nil {
			var a Actor
			if json.Unmarshal([]byte(v), &a) == nil {
				return &a, nil
			}
		}
		// Cache errors must never break the request; fall through to the store.
	}

	a, err := r.lookup(ctx, id)
	if err != nil {
		return nil, err
	}

	if r.cache != nil {
		if a != nil {
			if b, mErr := json.Marshal(a); mErr == nil {
				_ = r.cache.Set(ctx, cache.ActorKey(id), string(b), positiveTTL)
			}
		}
		if a == nil {
			_ = r.cache.Set(ctx, cache.ActorKey(id), nullSentinel, negativeTTL)
		}
	}
	return a, nil
}

// ResolveMany resolves a set of ids, returning a map from id to its actor
// (nil for unresolved ids). Duplicate ids are resolved once.
func (r *Resolver) ResolveMany(ctx context.Context, ids []string) (map[string]*Actor, error) {
	out := make(map[string]*Actor, len(ids))
	for _, id := range ids {
		if _, done := out[id]; done {
			continue
		}
		a, err := r.Resolve(ctx, id)
		if err != nil {
			return nil, err
		}
		out[id] = a
	}
	return out, nil
}

// lookup queries the store: users first (user-priority), then service accounts.
func (r *Resolver) lookup(ctx context.Context, id string) (*Actor, error) {
	u, err := r.store.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if u != nil {
		a := &Actor{
			ID:       u.ID,
			Type:     KindUser,
			Name:     u.Name,
			Email:    u.Email,
			Disabled: u.Disabled,
		}
		if u.AvatarAssetID != nil {
			a.AvatarAssetID = *u.AvatarAssetID
		}
		return a, nil
	}

	sa, err := r.store.GetServiceAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	if sa != nil {
		a := &Actor{
			ID:       sa.ID,
			Type:     KindServiceAccount,
			Name:     sa.Name,
			Disabled: sa.Disabled,
		}
		if sa.AvatarAssetID != nil {
			a.AvatarAssetID = *sa.AvatarAssetID
		}
		return a, nil
	}

	return nil, nil
}
