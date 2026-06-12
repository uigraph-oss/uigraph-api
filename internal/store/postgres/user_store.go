package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/org"
)

const userCols = `
	id, email, name, login,
	COALESCE(password_hash, ''),
	must_change_password,
	disabled,
	role,
	last_seen_at, created_at, updated_at`

func scanUser(row interface{ Scan(...any) error }) (org.User, error) {
	var u org.User
	var lastSeen sql.NullTime
	err := row.Scan(
		&u.ID, &u.Email, &u.Name, &u.Login,
		&u.PasswordHash, &u.MustChangePassword,
		&u.Disabled, &u.Role,
		&lastSeen, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return org.User{}, err
	}
	if lastSeen.Valid {
		u.LastSeenAt = &lastSeen.Time
	}
	return u, nil
}

func (d *DB) CreateUser(ctx context.Context, u org.User) error {
	const q = `
		INSERT INTO users
		    (id, email, name, login, password_hash, must_change_password, disabled, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7, $8, $9, $10)`

	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		u.ID, u.Email, u.Name, u.Login,
		u.PasswordHash, u.MustChangePassword, u.Disabled, u.Role,
		u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateUser: %w", err)
	}
	return nil
}

func (d *DB) UpsertUser(ctx context.Context, u org.User) error {
	const q = `
		INSERT INTO users
		    (id, email, name, login, password_hash, must_change_password, disabled, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7, $8, $9, $10)
		ON CONFLICT (email) DO UPDATE SET
		    name                = EXCLUDED.name,
		    login               = EXCLUDED.login,
		    password_hash       = EXCLUDED.password_hash,
		    must_change_password = EXCLUDED.must_change_password,
		    disabled            = EXCLUDED.disabled,
		    role                = EXCLUDED.role,
		    updated_at          = NOW()`

	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		u.ID, u.Email, u.Name, u.Login,
		u.PasswordHash, u.MustChangePassword, u.Disabled, u.Role,
		u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpsertUser: %w", err)
	}
	return nil
}

func (d *DB) GetUser(ctx context.Context, id string) (*org.User, error) {
	q := "SELECT " + userCols + " FROM users WHERE id = $1"
	u, err := scanUser(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetUser: %w", err)
	}
	return &u, nil
}

func (d *DB) GetUserByEmail(ctx context.Context, email string) (*org.User, error) {
	q := "SELECT " + userCols + " FROM users WHERE email = $1"
	u, err := scanUser(d.db.QueryRowContext(ctx, q, email))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetUserByEmail: %w", err)
	}
	return &u, nil
}

func (d *DB) GetUserByLogin(ctx context.Context, login string) (*org.User, error) {
	q := "SELECT " + userCols + " FROM users WHERE login = $1"
	u, err := scanUser(d.db.QueryRowContext(ctx, q, login))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetUserByLogin: %w", err)
	}
	return &u, nil
}

func (d *DB) ListUsers(ctx context.Context, orgID string) ([]org.User, error) {
	q := `SELECT ` + userCols + `
		FROM   users u
		JOIN   org_members om ON om.user_id = u.id
		WHERE  om.org_id = $1
		ORDER  BY u.name`

	rows, err := d.db.QueryContext(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListUsers: %w", err)
	}
	defer rows.Close()

	var out []org.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListUsers scan: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (d *DB) ListAllUsers(ctx context.Context) ([]org.User, error) {
	q := `SELECT ` + userCols + ` FROM users ORDER BY name`

	rows, err := d.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListAllUsers: %w", err)
	}
	defer rows.Close()

	var out []org.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListAllUsers scan: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (d *DB) AnyUserExists(ctx context.Context) (bool, error) {
	var n int
	if err := d.db.QueryRowContext(ctx, `SELECT 1 FROM users LIMIT 1`).Scan(&n); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("postgres: AnyUserExists: %w", err)
	}
	return true, nil
}

func (d *DB) UpdateUser(ctx context.Context, u org.User) error {
	const q = `
		UPDATE users
		SET    name                = $1,
		       login               = $2,
		       password_hash       = NULLIF($3, ''),
		       must_change_password = $4,
		       disabled            = $5,
		       role                = $6,
		       updated_at          = NOW()
		WHERE  id = $7`

	if _, err := d.db.ExecContext(ctx, q,
		u.Name, u.Login, u.PasswordHash, u.MustChangePassword, u.Disabled, u.Role, u.ID,
	); err != nil {
		return fmt.Errorf("postgres: UpdateUser: %w", err)
	}
	return nil
}

func (d *DB) DisableUser(ctx context.Context, id string) error {
	const q = `UPDATE users SET disabled = TRUE, updated_at = NOW() WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("postgres: DisableUser: %w", err)
	}
	return nil
}

func (d *DB) TouchUser(ctx context.Context, id string) error {
	const q = `UPDATE users SET last_seen_at = NOW() WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("postgres: TouchUser: %w", err)
	}
	return nil

}
