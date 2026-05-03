package repository

import (
	"context"
	"errors"
	"time"

	"vocabreview/backend/internal/domain"
)

var (
	ErrNotFound = errors.New("not found")
	ErrExpired  = errors.New("expired")
)

type VocabWithState struct {
	Item  domain.VocabItem
	State domain.ReviewState
}

type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

type AuthRepository interface {
	PutMagicLink(ctx context.Context, token domain.MagicLinkToken) error
	ConsumeMagicLink(ctx context.Context, token string, now time.Time, newUser domain.User, newSession domain.Session) (domain.User, domain.Session, error)
	GetSessionUser(ctx context.Context, token string) (domain.Session, domain.User, bool, error)
}

type VocabRepository interface {
	CreateVocab(ctx context.Context, item domain.VocabItem, state domain.ReviewState, job *domain.NotificationJob) error
	CreateCapturedVocab(ctx context.Context, item domain.VocabItem, state domain.ReviewState, capture domain.CaptureSource, job *domain.NotificationJob) error
	GetVocab(ctx context.Context, id string) (domain.VocabItem, bool, error)
	UpdateVocab(ctx context.Context, item domain.VocabItem) error
	ListVocabByUser(ctx context.Context, userID string) ([]VocabWithState, error)
	ListDueVocab(ctx context.Context, userID string, now time.Time) ([]VocabWithState, error)
	GetReviewState(ctx context.Context, vocabID string) (domain.ReviewState, bool, error)
	RecordReview(ctx context.Context, state domain.ReviewState, log domain.ReviewLog, job *domain.NotificationJob) error
}

type DeviceRepository interface {
	UpsertDeviceToken(ctx context.Context, token domain.DeviceToken) (domain.DeviceToken, error)
}

type NotificationRepository interface {
	ListNotificationJobs(ctx context.Context, userID string) ([]domain.NotificationJob, error)
}

type AppRepository interface {
	AuthRepository
	VocabRepository
	DeviceRepository
	NotificationRepository
	HealthChecker
}
