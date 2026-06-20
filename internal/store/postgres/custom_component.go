package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/componentlib"
)

// SaveCustomComponent upserts an org-scoped custom component and replaces its
// field set. Custom components carry a free-text category (category_text) and a
// NULL category_id.
func (d *DB) SaveCustomComponent(ctx context.Context, c componentlib.Component) error {
	const q = `
		INSERT INTO components
			(id, org_id, kind, type, name, slug, description, category_id, category_text,
			 tags, icon_key, is_active, sort_order, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULL,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO UPDATE SET
			type = EXCLUDED.type,
			name = EXCLUDED.name,
			slug = EXCLUDED.slug,
			description = EXCLUDED.description,
			category_text = EXCLUDED.category_text,
			tags = EXCLUDED.tags,
			is_active = EXCLUDED.is_active,
			sort_order = EXCLUDED.sort_order,
			updated_at = EXCLUDED.updated_at`
	now := time.Now().UTC()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	_, err := d.db.ExecContext(ctx, q,
		c.ID, c.OrgID, c.Kind, c.Type, c.Name, c.Slug, c.Description, c.CategoryName,
		componentlib.TagsJSON(c.Tags), c.IconKey, c.IsActive, c.Order,
		c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: SaveCustomComponent: %w", err)
	}
	return d.replaceComponentFields(ctx, c.ID, c.Fields)
}

func (d *DB) replaceComponentFields(ctx context.Context, componentID string, fields []componentlib.ComponentField) error {
	if _, err := d.db.ExecContext(ctx, `DELETE FROM component_fields WHERE component_id = $1`, componentID); err != nil {
		return fmt.Errorf("postgres: replaceComponentFields delete: %w", err)
	}
	const q = `
		INSERT INTO component_fields
			(id, component_id, label, type, required, readonly, options, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	for _, f := range fields {
		_, err := d.db.ExecContext(ctx, q,
			f.ID, componentID, f.Label, f.Type, f.Required, f.Readonly,
			componentlib.OptionsJSON(f.Options), f.Order,
		)
		if err != nil {
			return fmt.Errorf("postgres: replaceComponentFields insert: %w", err)
		}
	}
	return nil
}

func (d *DB) GetComponent(ctx context.Context, id string) (*componentlib.Component, error) {
	const q = `
		SELECT c.id, c.org_id, c.kind, c.type, c.name, c.slug, c.description,
		       COALESCE(c.category_id, ''), COALESCE(cat.name, c.category_text, ''),
		       c.tags, c.icon_key, c.is_active, c.sort_order, c.created_at, c.updated_at
		FROM components c
		LEFT JOIN component_categories cat ON cat.id = c.category_id
		WHERE c.id = $1`
	c, err := scanComponent(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetComponent: %w", err)
	}
	fields, err := d.listComponentFields(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	c.Fields = fields
	return &c, nil
}

func (d *DB) ListCustomComponents(ctx context.Context, orgID string) ([]componentlib.Component, error) {
	const q = `
		SELECT c.id, c.org_id, c.kind, c.type, c.name, c.slug, c.description,
		       COALESCE(c.category_id, ''), COALESCE(cat.name, c.category_text, ''),
		       c.tags, c.icon_key, c.is_active, c.sort_order, c.created_at, c.updated_at
		FROM components c
		LEFT JOIN component_categories cat ON cat.id = c.category_id
		WHERE c.org_id = $1
		ORDER BY c.sort_order ASC, c.name ASC`
	rows, err := d.db.QueryContext(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListCustomComponents: %w", err)
	}
	defer rows.Close()

	var comps []componentlib.Component
	for rows.Next() {
		c, err := scanComponent(rows)
		if err != nil {
			return nil, err
		}
		comps = append(comps, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: ListCustomComponents rows: %w", err)
	}

	for i := range comps {
		fields, err := d.listComponentFields(ctx, comps[i].ID)
		if err != nil {
			return nil, err
		}
		comps[i].Fields = fields
	}
	return comps, nil
}

func (d *DB) DeleteComponent(ctx context.Context, id string) error {
	if _, err := d.db.ExecContext(ctx, `DELETE FROM component_fields WHERE component_id = $1`, id); err != nil {
		return fmt.Errorf("postgres: DeleteComponent fields: %w", err)
	}
	if _, err := d.db.ExecContext(ctx, `DELETE FROM components WHERE id = $1`, id); err != nil {
		return fmt.Errorf("postgres: DeleteComponent: %w", err)
	}
	return nil
}
