package bootstrap

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/uigraph/app/assets"
	"github.com/uigraph/app/internal/componentcatalog"
	"github.com/uigraph/app/internal/storage"
)

// ComponentStore seeds component catalog rows and uploads icon SVGs.
type ComponentStore interface {
	componentcatalog.Store
}

// SeedComponents loads the embedded JSON manifest into Postgres and uploads icon
// SVGs to object storage. Safe to call on every startup.
func SeedComponents(ctx context.Context, s ComponentStore, st storage.Client) error {
	manifest, err := componentcatalog.LoadManifest()
	if err != nil {
		return fmt.Errorf("bootstrap: load component manifest: %w", err)
	}

	for _, cat := range componentcatalog.ExtractCategories(manifest) {
		if err := s.UpsertComponentCategory(ctx, cat); err != nil {
			return fmt.Errorf("bootstrap: upsert category %q: %w", cat.ID, err)
		}
	}

	for _, c := range manifest {
		if err := s.UpsertComponent(ctx, c); err != nil {
			return fmt.Errorf("bootstrap: upsert component %q: %w", c.ID, err)
		}
		for _, f := range c.Fields {
			if err := s.UpsertComponentField(ctx, f); err != nil {
				return fmt.Errorf("bootstrap: upsert field %q: %w", f.ID, err)
			}
		}
	}

	if err := uploadComponentIcons(ctx, s, st, manifest); err != nil {
		return err
	}

	slog.InfoContext(ctx, "component catalog seeded", "count", len(manifest))
	return nil
}

func uploadComponentIcons(ctx context.Context, s ComponentStore, st storage.Client, manifest []componentcatalog.Component) error {
	iconBySlug := map[string]string{}
	for _, c := range manifest {
		iconBySlug[componentcatalog.IconSlug(c)] = c.ID
	}

	return fs.WalkDir(assets.ComponentIcons, "component-icons", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".svg") {
			return nil
		}
		slug := strings.TrimSuffix(filepath.Base(path), ".svg")
		componentID, ok := iconBySlug[slug]
		if !ok {
			slog.WarnContext(ctx, "bootstrap: skipping unmapped icon", "slug", slug)
			return nil
		}

		data, err := assets.ComponentIcons.ReadFile(path)
		if err != nil {
			return fmt.Errorf("bootstrap: read icon %q: %w", path, err)
		}

		key := storage.ComponentIconKey(slug)
		if err := st.Upload(ctx, key, "image/svg+xml", bytes.NewReader(data), int64(len(data))); err != nil {
			return fmt.Errorf("bootstrap: upload icon %q: %w", slug, err)
		}
		if err := s.UpdateComponentIconKey(ctx, componentID, key); err != nil {
			return fmt.Errorf("bootstrap: update icon key for %q: %w", componentID, err)
		}
		return nil
	})
}
