package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/onesine/nevup-backend/internal/domain"
)

type User struct {
	UserID    string  `json:"userId"`
	Name      string  `json:"name"`
	Role      string  `json:"role"`
	Pathology *string `json:"pathology"`
}

type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

func (s *UserStore) GetByID(ctx context.Context, userID string) (User, error) {
	const sql = `SELECT user_id, name, role, pathology FROM users WHERE user_id = $1`
	var u User
	err := s.pool.QueryRow(ctx, sql, userID).Scan(&u.UserID, &u.Name, &u.Role, &u.Pathology)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, domain.ErrNotFound
	}
	return u, err
}
