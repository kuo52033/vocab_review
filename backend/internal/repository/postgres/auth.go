package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

func (s *Store) PutMagicLink(ctx context.Context, token domain.MagicLinkToken) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO magic_links (token, email, expires_at)
		VALUES ($1, $2, $3)
	`, token.Token, token.Email, token.ExpiresAt.UTC())
	return err
}

func (s *Store) ConsumeMagicLink(ctx context.Context, token string, now time.Time, newUser domain.User, newSession domain.Session) (domain.User, domain.Session, error) {
	var user domain.User
	var session domain.Session

	err := withTx(ctx, s.pool, func(tx pgx.Tx) error {
		var email string
		var expiresAt time.Time
		err := tx.QueryRow(ctx, `
			SELECT email, expires_at
			FROM magic_links
			WHERE token = $1
			FOR UPDATE
		`, token).Scan(&email, &expiresAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.ErrNotFound
		}
		if err != nil {
			return err
		}
		if now.After(expiresAt) {
			return repository.ErrExpired
		}

		err = tx.QueryRow(ctx, `
			SELECT id, email, created_at
			FROM users
			WHERE email = $1
		`, email).Scan(&user.ID, &user.Email, &user.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			newUser.Email = email
			if _, err := tx.Exec(ctx, `
				INSERT INTO users (id, email, created_at)
				VALUES ($1, $2, $3)
			`, newUser.ID, newUser.Email, newUser.CreatedAt.UTC()); err != nil {
				return err
			}
			user = newUser
		} else if err != nil {
			return err
		}

		newSession.UserID = user.ID
		if _, err := tx.Exec(ctx, `
			INSERT INTO sessions (token, user_id, created_at, expires_at)
			VALUES ($1, $2, $3, $4)
		`, newSession.Token, newSession.UserID, newSession.CreatedAt.UTC(), newSession.ExpiresAt.UTC()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM magic_links WHERE token = $1`, token); err != nil {
			return err
		}
		session = newSession
		return nil
	})
	return user, session, err
}

func (s *Store) GetSessionUser(ctx context.Context, token string) (domain.Session, domain.User, bool, error) {
	var session domain.Session
	var user domain.User
	err := s.pool.QueryRow(ctx, `
		SELECT s.token, s.user_id, s.created_at, s.expires_at,
		       u.id, u.email, u.created_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token = $1
	`, token).Scan(
		&session.Token,
		&session.UserID,
		&session.CreatedAt,
		&session.ExpiresAt,
		&user.ID,
		&user.Email,
		&user.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.User{}, false, nil
	}
	if err != nil {
		return domain.Session{}, domain.User{}, false, err
	}
	return session, user, true, nil
}
