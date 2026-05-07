package postgres

import (
	"context"
	"fmt"
)

func (s *Store) HealthCheck(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	var versionCount int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM goose_db_version`).Scan(&versionCount); err != nil {
		return fmt.Errorf("schema not ready: %w", err)
	}

	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return fmt.Errorf("users table unavailable: %w", err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM vocab_items)`).Scan(&exists); err != nil {
		return fmt.Errorf("vocab_items table unavailable: %w", err)
	}
	return nil
}
