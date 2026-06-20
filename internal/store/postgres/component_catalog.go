package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/componentlib"
)

func (d *DB) UpsertComponentCategory(ctx context.Context, cat componentlib.Category) error {
	const q = `
		INSERT INTO component_categories
			(id, org_id, kind, name, slug, sort_order, is_active, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			slug = EXCLUDED.slug,
			sort_order = EXCLUDED.sort_order,
			is_active = EXCLUDED.is_active,
			updated_at = EXCLUDED.updated_at`
	now := time.Now().UTC()
	if cat.CreatedAt.IsZero() {
		cat.CreatedAt = now
	}
	cat.UpdatedAt = now
	_, err := d.db.ExecContext(ctx, q,
		cat.ID, cat.OrgID, cat.Kind, cat.Name, cat.Slug,
		cat.SortOrder, cat.IsActive, cat.CreatedAt, cat.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpsertComponentCategory: %w", err)
	}
	return nil
}

func (d *DB) UpsertComponent(ctx context.Context, c componentlib.Component) error {
	const q = `
		INSERT INTO components
			(id, org_id, kind, type, name, slug, description, category_id, tags,
			 icon_key, is_active, sort_order, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (id) DO UPDATE SET
			kind = EXCLUDED.kind,
			type = EXCLUDED.type,
			name = EXCLUDED.name,
			slug = EXCLUDED.slug,
			description = EXCLUDED.description,
			category_id = EXCLUDED.category_id,
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
		c.ID, c.OrgID, c.Kind, c.Type, c.Name, c.Slug, c.Description, c.CategoryID,
		componentlib.TagsJSON(c.Tags), c.IconKey, c.IsActive, c.Order,
		c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpsertComponent: %w", err)
	}
	return nil
}

func (d *DB) UpsertComponentField(ctx context.Context, f componentlib.ComponentField) error {
	const q = `
		INSERT INTO component_fields
			(id, component_id, label, type, required, readonly, options, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (id) DO UPDATE SET
			component_id = EXCLUDED.component_id,
			label = EXCLUDED.label,
			type = EXCLUDED.type,
			required = EXCLUDED.required,
			readonly = EXCLUDED.readonly,
			options = EXCLUDED.options,
			sort_order = EXCLUDED.sort_order`
	_, err := d.db.ExecContext(ctx, q,
		f.ID, f.ComponentID, f.Label, f.Type, f.Required, f.Readonly,
		componentlib.OptionsJSON(f.Options), f.Order,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpsertComponentField: %w", err)
	}
	return nil
}

func (d *DB) ListComponentsByKind(ctx context.Context, kind string) ([]componentlib.Component, error) {
	const q = `
		SELECT c.id, c.org_id, c.kind, c.type, c.name, c.slug, c.description,
		       c.category_id, cat.name, c.tags, c.icon_key, c.is_active, c.sort_order,
		       c.created_at, c.updated_at
		FROM components c
		JOIN component_categories cat ON cat.id = c.category_id
		WHERE c.kind = $1 AND c.org_id IS NULL
		ORDER BY c.sort_order ASC, c.name ASC`
	rows, err := d.db.QueryContext(ctx, q, kind)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListComponentsByKind: %w", err)
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
		return nil, fmt.Errorf("postgres: ListComponentsByKind rows: %w", err)
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

func (d *DB) listComponentFields(ctx context.Context, componentID string) ([]componentlib.ComponentField, error) {
	const q = `
		SELECT id, component_id, label, type, required, readonly, options, sort_order
		FROM component_fields
		WHERE component_id = $1
		ORDER BY sort_order ASC`
	rows, err := d.db.QueryContext(ctx, q, componentID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listComponentFields: %w", err)
	}
	defer rows.Close()

	var out []componentlib.ComponentField
	for rows.Next() {
		var f componentlib.ComponentField
		var opts []byte
		var readonly sql.NullBool
		if err := rows.Scan(&f.ID, &f.ComponentID, &f.Label, &f.Type, &f.Required, &readonly, &opts, &f.Order); err != nil {
			return nil, fmt.Errorf("postgres: scan field: %w", err)
		}
		if readonly.Valid {
			v := readonly.Bool
			f.Readonly = &v
		}
		if len(opts) > 0 {
			_ = json.Unmarshal(opts, &f.Options)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (d *DB) UpdateComponentIconKey(ctx context.Context, id, iconKey string) error {
	const q = `UPDATE components SET icon_key = $2, updated_at = NOW() WHERE id = $1`
	_, err := d.db.ExecContext(ctx, q, id, iconKey)
	if err != nil {
		return fmt.Errorf("postgres: UpdateComponentIconKey: %w", err)
	}
	return nil
}

func (d *DB) CountComponents(ctx context.Context) (int, error) {
	var n int
	err := d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM components`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("postgres: CountComponents: %w", err)
	}
	return n, nil
}

type componentScanner interface {
	Scan(dest ...any) error
}

func scanComponent(row componentScanner) (componentlib.Component, error) {
	var c componentlib.Component
	var tags []byte
	var iconKey sql.NullString
	var orgID sql.NullString
	if err := row.Scan(
		&c.ID, &orgID, &c.Kind, &c.Type, &c.Name, &c.Slug, &c.Description,
		&c.CategoryID, &c.CategoryName, &tags, &iconKey, &c.IsActive, &c.Order,
		&c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return c, fmt.Errorf("postgres: scanComponent: %w", err)
	}
	if orgID.Valid {
		c.OrgID = &orgID.String
	}
	if len(tags) > 0 {
		_ = json.Unmarshal(tags, &c.Tags)
	}
	if c.Tags == nil {
		c.Tags = []string{}
	}
	if iconKey.Valid {
		c.IconKey = &iconKey.String
	}
	return c, nil
}
