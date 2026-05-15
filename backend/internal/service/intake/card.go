package intake

import (
	"errors"
	"strings"
	"time"

	"vocabreview/backend/internal/domain"
)

const (
	notificationStatusPending = "pending"
	reviewReminderMessage     = "Time to review your vocabulary."
	captureSourceChrome       = "chrome-extension"
)

type IDs struct {
	VocabItemID       string
	CaptureSourceID   string
	NotificationJobID string
}

type VocabInput struct {
	Term            string
	Meaning         string
	ExampleSentence string
	PartOfSpeech    domain.PartOfSpeech
	SourceText      string
	SourceURL       string
	Notes           string
}

type CaptureInput struct {
	Term            string
	Meaning         string
	ExampleSentence string
	PartOfSpeech    domain.PartOfSpeech
	Selection       string
	PageTitle       string
	PageURL         string
	Notes           string
}

type Card struct {
	Item            domain.VocabItem
	State           domain.ReviewState
	NotificationJob *domain.NotificationJob
}

type CapturedCard struct {
	Card
	Capture domain.CaptureSource
}

func NewVocabCard(userID string, input VocabInput, ids IDs, now time.Time) (Card, error) {
	term := strings.TrimSpace(input.Term)
	if term == "" {
		return Card{}, errors.New("term is required")
	}
	item := domain.VocabItem{
		ID:              ids.VocabItemID,
		UserID:          userID,
		Term:            term,
		Meaning:         strings.TrimSpace(input.Meaning),
		ExampleSentence: strings.TrimSpace(input.ExampleSentence),
		PartOfSpeech:    input.PartOfSpeech,
		SourceText:      strings.TrimSpace(input.SourceText),
		SourceURL:       strings.TrimSpace(input.SourceURL),
		Notes:           strings.TrimSpace(input.Notes),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	state := initialReviewState(userID, item.ID, now)
	return Card{
		Item:            item,
		State:           state,
		NotificationJob: pendingReviewJob(userID, item.ID, ids.NotificationJobID, now),
	}, nil
}

func NewCapturedCard(userID string, input CaptureInput, ids IDs, now time.Time) (CapturedCard, error) {
	card, err := NewVocabCard(userID, VocabInput{
		Term:            input.Term,
		Meaning:         input.Meaning,
		ExampleSentence: input.ExampleSentence,
		PartOfSpeech:    input.PartOfSpeech,
		SourceText:      input.Selection,
		SourceURL:       input.PageURL,
		Notes:           input.Notes,
	}, ids, now)
	if err != nil {
		return CapturedCard{}, err
	}

	capture := domain.CaptureSource{
		ID:          ids.CaptureSourceID,
		UserID:      userID,
		VocabItemID: card.Item.ID,
		Source:      captureSourceChrome,
		Selection:   input.Selection,
		PageTitle:   input.PageTitle,
		PageURL:     input.PageURL,
		CreatedAt:   now,
	}

	return CapturedCard{Card: card, Capture: capture}, nil
}

func initialReviewState(userID string, vocabItemID string, now time.Time) domain.ReviewState {
	return domain.ReviewState{
		VocabItemID:     vocabItemID,
		UserID:          userID,
		Status:          domain.ReviewStatusNew,
		EaseFactor:      2.5,
		IntervalDays:    0,
		RepetitionCount: 0,
		NextDueAt:       now,
	}
}

func pendingReviewJob(userID string, vocabItemID string, jobID string, scheduledAt time.Time) *domain.NotificationJob {
	return &domain.NotificationJob{
		ID:          jobID,
		UserID:      userID,
		VocabItemID: vocabItemID,
		ScheduledAt: scheduledAt,
		Status:      notificationStatusPending,
		Message:     reviewReminderMessage,
	}
}
