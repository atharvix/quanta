package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/atharvix/quanta/apps/backend/internal/database"
	"github.com/atharvix/quanta/apps/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// UserRepository defines persistence operations for user accounts.
type UserRepository interface {
	Create(ctx context.Context, user domain.User) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
	Count(ctx context.Context) (int, error)
	List(ctx context.Context) ([]domain.User, error)
	UpdateRole(ctx context.Context, id string, role domain.Role) (*domain.User, error)
	CountAdmins(ctx context.Context) (int, error)
}

// PgxUserRepository implements UserRepository using pgxpool with parameterized queries.
type PgxUserRepository struct {
	db *database.DB
}

// NewPgxUserRepository constructs a UserRepository instance tied to the thread-safe database container.
func NewPgxUserRepository(db *database.DB) *PgxUserRepository {
	return &PgxUserRepository{db: db}
}

// Create inserts a new user record. Returns ErrEmailAlreadyExists on duplicate email.
func (r *PgxUserRepository) Create(ctx context.Context, user domain.User) (*domain.User, error) {
	pool := r.db.Pool()
	if pool == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, email, password_hash, full_name, role, created_at;
	`

	var saved domain.User
	var role string
	err := pool.QueryRow(
		timeoutCtx,
		query,
		user.Email,
		user.PasswordHash,
		user.FullName,
		string(user.Role),
	).Scan(
		&saved.ID,
		&saved.Email,
		&saved.PasswordHash,
		&saved.FullName,
		&role,
		&saved.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "email") {
			return nil, domain.ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}
	saved.Role = domain.Role(role)

	return &saved, nil
}

// GetByEmail retrieves a user by email address. Returns ErrNotFound if no match.
func (r *PgxUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	pool := r.db.Pool()
	if pool == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		SELECT id::text, email, password_hash, full_name, role, created_at
		FROM users
		WHERE email = $1;
	`

	var user domain.User
	var role string
	err := pool.QueryRow(timeoutCtx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&role,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query user by email: %w", err)
	}
	user.Role = domain.Role(role)

	return &user, nil
}

// GetByID retrieves a user by UUID. Returns ErrNotFound if no match.
func (r *PgxUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	pool := r.db.Pool()
	if pool == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		SELECT id::text, email, password_hash, full_name, role, created_at
		FROM users
		WHERE id = $1;
	`

	var user domain.User
	var role string
	err := pool.QueryRow(timeoutCtx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&role,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to query user by id: %w", err)
	}
	user.Role = domain.Role(role)

	return &user, nil
}

// Count returns the total number of registered users.
func (r *PgxUserRepository) Count(ctx context.Context) (int, error) {
	pool := r.db.Pool()
	if pool == nil {
		return 0, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var count int
	err := pool.QueryRow(timeoutCtx, "SELECT COUNT(*) FROM users;").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}

// List retrieves all registered users ordered by creation date ascending.
func (r *PgxUserRepository) List(ctx context.Context) ([]domain.User, error) {
	pool := r.db.Pool()
	if pool == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		SELECT id::text, email, password_hash, full_name, role, created_at
		FROM users
		ORDER BY created_at ASC;
	`

	rows, err := pool.Query(timeoutCtx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		var role string
		if err := rows.Scan(
			&u.ID,
			&u.Email,
			&u.PasswordHash,
			&u.FullName,
			&role,
			&u.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}
		u.Role = domain.Role(role)
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading user rows: %w", err)
	}

	if users == nil {
		users = []domain.User{}
	}

	return users, nil
}

// UpdateRole modifies the role of a user identified by UUID.
func (r *PgxUserRepository) UpdateRole(ctx context.Context, id string, role domain.Role) (*domain.User, error) {
	pool := r.db.Pool()
	if pool == nil {
		return nil, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	const query = `
		UPDATE users
		SET role = $1
		WHERE id = $2
		RETURNING id::text, email, password_hash, full_name, role, created_at;
	`

	var updated domain.User
	var roleStr string
	err := pool.QueryRow(timeoutCtx, query, string(role), id).Scan(
		&updated.ID,
		&updated.Email,
		&updated.PasswordHash,
		&updated.FullName,
		&roleStr,
		&updated.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to update user role: %w", err)
	}
	updated.Role = domain.Role(roleStr)

	return &updated, nil
}

// CountAdmins returns the total number of users currently holding the admin role.
func (r *PgxUserRepository) CountAdmins(ctx context.Context) (int, error) {
	pool := r.db.Pool()
	if pool == nil {
		return 0, domain.ErrDatabaseUnavailable
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var count int
	err := pool.QueryRow(timeoutCtx, "SELECT COUNT(*) FROM users WHERE role = 'admin';").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count admin users: %w", err)
	}

	return count, nil
}
