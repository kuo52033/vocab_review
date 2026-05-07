package postgres

import (
	"context"

	"vocabreview/backend/internal/domain"
)

func (s *Store) UpsertDeviceToken(ctx context.Context, token domain.DeviceToken) (domain.DeviceToken, error) {
	var stored domain.DeviceToken
	err := s.pool.QueryRow(ctx, `
		INSERT INTO device_tokens (id, user_id, platform, token, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, token)
		DO UPDATE SET
			platform = EXCLUDED.platform,
			updated_at = EXCLUDED.updated_at
		RETURNING id, user_id, platform, token, created_at, updated_at
	`, token.ID, token.UserID, token.Platform, token.Token, token.CreatedAt.UTC(), token.UpdatedAt.UTC()).Scan(
		&stored.ID,
		&stored.UserID,
		&stored.Platform,
		&stored.Token,
		&stored.CreatedAt,
		&stored.UpdatedAt,
	)
	return stored, err
}
