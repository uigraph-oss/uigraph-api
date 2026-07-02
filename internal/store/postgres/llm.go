package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/llm"
)

func (d *DB) ListLLMModels(ctx context.Context) ([]llm.LLMModel, error) {
	const q = `
		SELECT id, model_id, provider, display_name,
		       input_cost_per_million, output_cost_per_million,
		       is_active, created_at, updated_at
		FROM llm_models WHERE is_active = TRUE ORDER BY provider, display_name`
	rows, err := d.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListLLMModels: %w", err)
	}
	defer rows.Close()
	var out []llm.LLMModel
	for rows.Next() {
		m, scanErr := scanLLMModel(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres: ListLLMModels scan: %w", scanErr)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (d *DB) GetLLMModel(ctx context.Context, id string) (*llm.LLMModel, error) {
	const q = `
		SELECT id, model_id, provider, display_name,
		       input_cost_per_million, output_cost_per_million,
		       is_active, created_at, updated_at
		FROM llm_models WHERE id = $1`
	m, err := scanLLMModel(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetLLMModel: %w", err)
	}
	return &m, nil
}

func (d *DB) CreateLLMModel(ctx context.Context, m llm.LLMModel) error {
	const q = `
		INSERT INTO llm_models
			(id, model_id, provider, display_name, input_cost_per_million, output_cost_per_million, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	now := time.Now().UTC()
	_, err := d.db.ExecContext(ctx, q,
		m.ID, m.ModelID, m.Provider, m.DisplayName,
		m.InputCostPerMillion, m.OutputCostPerMillion, now, now,
	)
	return wrapErr("CreateLLMModel", err)
}

func (d *DB) UpdateLLMModel(ctx context.Context, m llm.LLMModel) error {
	const q = `
		UPDATE llm_models
		SET display_name=$1, input_cost_per_million=$2, output_cost_per_million=$3, updated_at=$4
		WHERE id=$5`
	_, err := d.db.ExecContext(ctx, q,
		m.DisplayName, m.InputCostPerMillion, m.OutputCostPerMillion,
		time.Now().UTC(), m.ID,
	)
	return wrapErr("UpdateLLMModel", err)
}

func (d *DB) DeactivateLLMModel(ctx context.Context, id string) error {
	const q = `UPDATE llm_models SET is_active=FALSE, updated_at=$1 WHERE id=$2`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), id)
	return wrapErr("DeactivateLLMModel", err)
}

func scanLLMModel(r interface{ Scan(...interface{}) error }) (llm.LLMModel, error) {
	var m llm.LLMModel
	return m, r.Scan(
		&m.ID, &m.ModelID, &m.Provider, &m.DisplayName,
		&m.InputCostPerMillion, &m.OutputCostPerMillion,
		&m.IsActive, &m.CreatedAt, &m.UpdatedAt,
	)
}
