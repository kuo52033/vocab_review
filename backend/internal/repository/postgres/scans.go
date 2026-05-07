package postgres

import (
	"time"

	"github.com/jackc/pgx/v5"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

type scanner interface {
	Scan(dest ...any) error
}

func scanVocab(row scanner) (domain.VocabItem, error) {
	var item domain.VocabItem
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.Term,
		&item.Kind,
		&item.Meaning,
		&item.ExampleSentence,
		&item.SourceText,
		&item.SourceURL,
		&item.Notes,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.ArchivedAt,
	); err != nil {
		return domain.VocabItem{}, err
	}
	return item, nil
}

func scanReviewState(row scanner) (domain.ReviewState, error) {
	var state domain.ReviewState
	if err := row.Scan(
		&state.VocabItemID,
		&state.UserID,
		&state.Status,
		&state.EaseFactor,
		&state.IntervalDays,
		&state.RepetitionCount,
		&state.LastReviewedAt,
		&state.NextDueAt,
		&state.ConsecutiveAgain,
	); err != nil {
		return domain.ReviewState{}, err
	}
	return state, nil
}

func scanVocabWithStates(rows pgx.Rows) ([]repository.VocabWithState, error) {
	result := make([]repository.VocabWithState, 0)
	for rows.Next() {
		var item domain.VocabItem
		var state domain.ReviewState
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Term,
			&item.Kind,
			&item.Meaning,
			&item.ExampleSentence,
			&item.SourceText,
			&item.SourceURL,
			&item.Notes,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.ArchivedAt,
			&state.VocabItemID,
			&state.UserID,
			&state.Status,
			&state.EaseFactor,
			&state.IntervalDays,
			&state.RepetitionCount,
			&state.LastReviewedAt,
			&state.NextDueAt,
			&state.ConsecutiveAgain,
		); err != nil {
			return nil, err
		}
		result = append(result, repository.VocabWithState{Item: item, State: state})
	}
	return result, rows.Err()
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
