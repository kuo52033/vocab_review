package postgres

import (
	"context"
	"fmt"
)

func (s *Store) HealthCheck(ctx context.Context) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = 'goose_db_version'
		)
	`).Scan(&exists); err != nil {
		return fmt.Errorf("schema not ready: %w", err)
	}
	if !exists {
		return fmt.Errorf("schema not ready: goose_db_version table unavailable")
	}
	return nil
}
