// Package asset resolves a bare asset id (as stored in preview_asset_id /
// asset_id / screenshot_asset_id columns) into a presigned GET URL the client
// can hit directly. The object key is always storage.AssetKey(id). Presigning
// is local crypto, so the URL is cached in Redis to keep it stable across
// requests (so the browser's image cache stays warm), not to save compute.
package asset

import (
	"context"
	"time"

	"github.com/uigraph/app/internal/cache"
	"github.com/uigraph/app/internal/storage"
)

// presignTTL is how long a minted presigned URL is valid.
// cacheTTL is how long the URL is cached; it is shorter than presignTTL so a
// cache-served URL always has remaining life.
const (
	presignTTL = time.Hour
	cacheTTL   = 50 * time.Minute
)

// Resolver mints presigned URLs for asset ids, cached in Redis.
type Resolver struct {
	storage storage.Client
	cache   cache.Client // may be nil
}

func New(st storage.Client, c cache.Client) *Resolver {
	return &Resolver{storage: st, cache: c}
}

// Resolve returns a presigned GET URL for asset id.
func (r *Resolver) Resolve(ctx context.Context, id string) (string, error) {
	if r.cache != nil {
		v, err := r.cache.Get(ctx, cache.AssetURLKey(id))
		if err == nil {
			return v, nil
		}
		// Cache errors must never break the request; fall through to presigning.
	}

	url, err := r.storage.PresignURL(ctx, storage.AssetKey(id), presignTTL)
	if err != nil {
		return "", err
	}

	if r.cache != nil {
		_ = r.cache.Set(ctx, cache.AssetURLKey(id), url, cacheTTL)
	}
	return url, nil
}

// ResolveMany resolves a set of ids, returning a map from id to its presigned
// URL. Duplicate ids are resolved once.
func (r *Resolver) ResolveMany(ctx context.Context, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		if _, done := out[id]; done {
			continue
		}
		url, err := r.Resolve(ctx, id)
		if err != nil {
			return nil, err
		}
		out[id] = url
	}
	return out, nil
}
