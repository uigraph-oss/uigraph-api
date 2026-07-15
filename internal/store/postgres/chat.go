package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/chat"
)

func (d *DB) CreateChatSession(ctx context.Context, s chat.ChatSession) error {
	const q = `
		INSERT INTO chat_sessions
			(id, org_id, owner_user_id, title, is_pinned, created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	now := time.Now().UTC()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		s.ID, s.OrgID, s.OwnerUserID, s.Title, s.IsPinned, s.CreatedBy, s.UpdatedBy, s.CreatedAt, s.UpdatedAt,
	)
	return wrapErr("CreateChatSession", err)
}

func (d *DB) GetChatSession(ctx context.Context, id string) (*chat.ChatSession, error) {
	const q = `
		SELECT id, org_id, owner_user_id, title, is_pinned,
		       (SELECT COUNT(*) FROM chat_messages m WHERE m.chat_session_id = s.id) AS message_count,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM chat_sessions s
		WHERE id = $1`
	sess, err := scanChatSession(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetChatSession: %w", err)
	}
	return &sess, nil
}

func (d *DB) ListChatSessions(ctx context.Context, orgID, ownerUserID string) ([]chat.ChatSession, error) {
	const q = `
		SELECT id, org_id, owner_user_id, title, is_pinned,
		       (SELECT COUNT(*) FROM chat_messages m WHERE m.chat_session_id = s.id) AS message_count,
		       created_by, updated_by, created_at, updated_at, deleted_at, deleted_by
		FROM chat_sessions s
		WHERE org_id = $1 AND owner_user_id = $2 AND deleted_at IS NULL
		ORDER BY is_pinned DESC, updated_at DESC`
	rows, err := d.db.QueryContext(ctx, q, orgID, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListChatSessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []chat.ChatSession
	for rows.Next() {
		sess, scanErr := scanChatSession(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres: ListChatSessions scan: %w", scanErr)
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

func (d *DB) UpdateChatSession(ctx context.Context, s chat.ChatSession) error {
	const q = `
		UPDATE chat_sessions
		SET title=$1, is_pinned=$2, updated_by=$3, updated_at=$4
		WHERE id=$5 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q,
		s.Title, s.IsPinned, s.UpdatedBy, time.Now().UTC(), s.ID,
	)
	return wrapErr("UpdateChatSession", err)
}

func (d *DB) SoftDeleteChatSession(ctx context.Context, id, deletedBy string) error {
	const q = `UPDATE chat_sessions SET deleted_at=$1, deleted_by=$2 WHERE id=$3 AND deleted_at IS NULL`
	_, err := d.db.ExecContext(ctx, q, time.Now().UTC(), deletedBy, id)
	return wrapErr("SoftDeleteChatSession", err)
}

func scanChatSession(row interface{ Scan(...any) error }) (chat.ChatSession, error) {
	var s chat.ChatSession
	err := row.Scan(
		&s.ID, &s.OrgID, &s.OwnerUserID, &s.Title, &s.IsPinned, &s.MessageCount,
		&s.CreatedBy, &s.UpdatedBy, &s.CreatedAt, &s.UpdatedAt, &s.DeletedAt, &s.DeletedBy,
	)
	return s, err
}

func (d *DB) CreateChatMessage(ctx context.Context, m chat.ChatMessage) error {
	const q = `
		INSERT INTO chat_messages
			(id, org_id, chat_session_id, role, content, parts, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	var parts any
	if len(m.Parts) > 0 {
		parts = []byte(m.Parts)
	}
	if _, err := d.db.ExecContext(ctx, q,
		m.ID, m.OrgID, m.ChatSessionID, m.Role, m.Content, parts, m.CreatedAt,
	); err != nil {
		return wrapErr("CreateChatMessage", err)
	}
	_, err := d.db.ExecContext(ctx,
		`UPDATE chat_sessions SET updated_at=$1 WHERE id=$2 AND deleted_at IS NULL`,
		m.CreatedAt, m.ChatSessionID,
	)
	return wrapErr("CreateChatMessage(bump session)", err)
}

func (d *DB) ListChatMessages(ctx context.Context, chatSessionID string) ([]chat.ChatMessage, error) {
	const q = `
		SELECT id, org_id, chat_session_id, role, content, parts, created_at
		FROM chat_messages
		WHERE chat_session_id = $1
		ORDER BY created_at ASC`
	rows, err := d.db.QueryContext(ctx, q, chatSessionID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListChatMessages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []chat.ChatMessage
	for rows.Next() {
		m, scanErr := scanChatMessage(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("postgres: ListChatMessages scan: %w", scanErr)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanChatMessage(row interface{ Scan(...any) error }) (chat.ChatMessage, error) {
	var m chat.ChatMessage
	var parts []byte
	err := row.Scan(
		&m.ID, &m.OrgID, &m.ChatSessionID, &m.Role, &m.Content, &parts, &m.CreatedAt,
	)
	m.Parts = parts
	return m, err
}
