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
	var audioID, audioStorageKey, audioStatus, audioProvider, audioModel, audioVoice, audioFormat string
	var audioSpeed float64
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.Term,
		&item.Meaning,
		&item.Chinese,
		&item.ExampleSentence,
		&item.PartOfSpeech,
		&item.SourceText,
		&item.SourceURL,
		&item.Notes,
		&audioID,
		&audioStorageKey,
		&audioStatus,
		&audioProvider,
		&audioModel,
		&audioVoice,
		&audioSpeed,
		&audioFormat,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.ArchivedAt,
	); err != nil {
		return domain.VocabItem{}, err
	}
	item.AudioID = audioID
	item.Audio = audioFromScan(audioID, audioStorageKey, audioStatus, audioProvider, audioModel, audioVoice, audioSpeed, audioFormat)
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
		var audioID, audioStorageKey, audioStatus, audioProvider, audioModel, audioVoice, audioFormat string
		var audioSpeed float64
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Term,
			&item.Meaning,
			&item.Chinese,
			&item.ExampleSentence,
			&item.PartOfSpeech,
			&item.SourceText,
			&item.SourceURL,
			&item.Notes,
			&audioID,
			&audioStorageKey,
			&audioStatus,
			&audioProvider,
			&audioModel,
			&audioVoice,
			&audioSpeed,
			&audioFormat,
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
		item.AudioID = audioID
		item.Audio = audioFromScan(audioID, audioStorageKey, audioStatus, audioProvider, audioModel, audioVoice, audioSpeed, audioFormat)
		result = append(result, repository.VocabWithState{Item: item, State: state})
	}
	return result, rows.Err()
}

func audioFromScan(id, storageKey, status, provider, model, voice string, speed float64, outputFormat string) *domain.VocabAudio {
	if status == "" {
		return nil
	}
	return &domain.VocabAudio{
		ID:           id,
		Provider:     provider,
		Model:        model,
		Voice:        voice,
		Speed:        speed,
		OutputFormat: outputFormat,
		StorageKey:   storageKey,
		Status:       status,
	}
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
