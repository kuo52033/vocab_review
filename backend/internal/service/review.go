package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

type App struct {
	store repository.AppRepository
	clock clock.Clock
}

func NewApp(store repository.AppRepository, appClock clock.Clock) *App {
	return &App{store: store, clock: appClock}
}

type AuthResult struct {
	User        domain.User    `json:"user"`
	Session     domain.Session `json:"session"`
	RedirectURL string         `json:"redirect_url"`
}

func (a *App) RequestMagicLink(email, baseURL string) (map[string]string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, errors.New("email is required")
	}

	token := domain.MagicLinkToken{
		Token:     newID("ml"),
		Email:     email,
		ExpiresAt: a.clock.Now().Add(15 * time.Minute),
	}
	if err := a.store.PutMagicLink(context.Background(), token); err != nil {
		return nil, err
	}

	return map[string]string{
		"token":            token.Token,
		"verification_url": strings.TrimRight(baseURL, "/") + "/?token=" + token.Token,
		"expires_at":       token.ExpiresAt.Format(time.RFC3339),
	}, nil
}

func (a *App) VerifyMagicLink(token string) (AuthResult, error) {
	now := a.clock.Now()
	newUser := domain.User{
		ID:        newID("usr"),
		CreatedAt: now,
	}
	newSession := domain.Session{
		Token:     newID("sess"),
		CreatedAt: now,
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}
	user, session, err := a.store.ConsumeMagicLink(context.Background(), token, now, newUser, newSession)
	if errors.Is(err, repository.ErrNotFound) {
		return AuthResult{}, errors.New("invalid token")
	}
	if errors.Is(err, repository.ErrExpired) {
		return AuthResult{}, errors.New("token expired")
	}
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		User:        user,
		Session:     session,
		RedirectURL: "/auth/success?session_token=" + session.Token,
	}, nil
}

func (a *App) Session(token string) (domain.Session, domain.User, error) {
	session, user, ok, err := a.store.GetSessionUser(context.Background(), token)
	if err != nil {
		return domain.Session{}, domain.User{}, err
	}
	if !ok {
		return domain.Session{}, domain.User{}, errors.New("unauthorized")
	}
	if session.ExpiresAt.Before(a.clock.Now()) {
		return domain.Session{}, domain.User{}, errors.New("session expired")
	}
	return session, user, nil
}

type CreateVocabInput struct {
	Term            string          `json:"term"`
	Kind            domain.CardKind `json:"kind"`
	Meaning         string          `json:"meaning"`
	ExampleSentence string          `json:"example_sentence"`
	SourceText      string          `json:"source_text"`
	SourceURL       string          `json:"source_url"`
	Notes           string          `json:"notes"`
}

func (a *App) CreateVocab(userID string, input CreateVocabInput) (domain.VocabItem, domain.ReviewState, error) {
	if strings.TrimSpace(input.Term) == "" {
		return domain.VocabItem{}, domain.ReviewState{}, errors.New("term is required")
	}
	if input.Kind == "" {
		input.Kind = domain.CardKindWord
	}
	now := a.clock.Now()
	item := domain.VocabItem{
		ID:              newID("voc"),
		UserID:          userID,
		Term:            strings.TrimSpace(input.Term),
		Kind:            input.Kind,
		Meaning:         strings.TrimSpace(input.Meaning),
		ExampleSentence: strings.TrimSpace(input.ExampleSentence),
		SourceText:      strings.TrimSpace(input.SourceText),
		SourceURL:       strings.TrimSpace(input.SourceURL),
		Notes:           strings.TrimSpace(input.Notes),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	state := domain.ReviewState{
		VocabItemID:     item.ID,
		UserID:          userID,
		Status:          domain.ReviewStatusNew,
		EaseFactor:      2.5,
		IntervalDays:    0,
		RepetitionCount: 0,
		NextDueAt:       now,
	}
	var job *domain.NotificationJob
	if !state.NextDueAt.After(now) {
		job = &domain.NotificationJob{
			ID:          newID("job"),
			UserID:      userID,
			VocabItemID: item.ID,
			ScheduledAt: now,
			Status:      "pending",
			Message:     "Time to review your vocabulary.",
		}
	}
	if err := a.store.CreateVocab(context.Background(), item, state, job); err != nil {
		return domain.VocabItem{}, domain.ReviewState{}, err
	}
	return item, state, nil
}

func (a *App) UpdateVocab(userID, id string, input CreateVocabInput) (domain.VocabItem, error) {
	item, ok, err := a.store.GetVocab(context.Background(), id)
	if err != nil {
		return domain.VocabItem{}, err
	}
	if !ok || item.UserID != userID {
		return domain.VocabItem{}, errors.New("vocab not found")
	}
	item.Term = strings.TrimSpace(defaultString(input.Term, item.Term))
	if input.Kind != "" {
		item.Kind = input.Kind
	}
	item.Meaning = strings.TrimSpace(defaultString(input.Meaning, item.Meaning))
	item.ExampleSentence = strings.TrimSpace(defaultString(input.ExampleSentence, item.ExampleSentence))
	item.SourceText = strings.TrimSpace(defaultString(input.SourceText, item.SourceText))
	item.SourceURL = strings.TrimSpace(defaultString(input.SourceURL, item.SourceURL))
	item.Notes = strings.TrimSpace(defaultString(input.Notes, item.Notes))
	item.UpdatedAt = a.clock.Now()
	if err := a.store.UpdateVocab(context.Background(), item); err != nil {
		return domain.VocabItem{}, err
	}
	return item, nil
}

func defaultString(next, current string) string {
	if strings.TrimSpace(next) == "" {
		return current
	}
	return next
}

type VocabWithState struct {
	Item  domain.VocabItem   `json:"item"`
	State domain.ReviewState `json:"state"`
}

func (a *App) ListVocab(userID string) ([]VocabWithState, error) {
	items, err := a.store.ListVocabByUser(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	result := make([]VocabWithState, 0, len(items))
	for _, item := range items {
		result = append(result, VocabWithState{Item: item.Item, State: item.State})
	}
	return result, nil
}

type DueCard struct {
	Item  domain.VocabItem   `json:"item"`
	State domain.ReviewState `json:"state"`
}

func (a *App) DueCards(userID string) ([]DueCard, error) {
	states, err := a.store.ListDueVocab(context.Background(), userID, a.clock.Now())
	if err != nil {
		return nil, err
	}
	result := make([]DueCard, 0, len(states))
	for _, state := range states {
		result = append(result, DueCard{Item: state.Item, State: state.State})
	}
	return result, nil
}

func (a *App) GradeReview(userID, vocabID string, grade domain.ReviewGrade) (domain.ReviewState, error) {
	state, ok, err := a.store.GetReviewState(context.Background(), vocabID)
	if err != nil {
		return domain.ReviewState{}, err
	}
	if !ok || state.UserID != userID {
		return domain.ReviewState{}, errors.New("review not found")
	}

	next := state
	now := a.clock.Now()
	next.LastReviewedAt = &now

	switch grade {
	case domain.ReviewGradeAgain:
		next.Status = domain.ReviewStatusLearning
		next.IntervalDays = 0
		next.RepetitionCount = 0
		next.ConsecutiveAgain++
		next.EaseFactor = math.Max(1.3, next.EaseFactor-0.2)
		next.NextDueAt = now.Add(4 * time.Hour)
	case domain.ReviewGradeHard:
		next.Status = domain.ReviewStatusReview
		next.ConsecutiveAgain = 0
		next.RepetitionCount++
		if next.RepetitionCount <= 1 {
			next.IntervalDays = 1
		} else {
			next.IntervalDays = max(1, int(math.Round(float64(max(1, state.IntervalDays))*1.2)))
		}
		next.EaseFactor = math.Max(1.3, next.EaseFactor-0.15)
		next.NextDueAt = now.Add(time.Duration(next.IntervalDays) * 24 * time.Hour)
	case domain.ReviewGradeGood:
		next.Status = domain.ReviewStatusReview
		next.ConsecutiveAgain = 0
		next.RepetitionCount++
		if next.RepetitionCount == 1 {
			next.IntervalDays = 1
		} else if next.RepetitionCount == 2 {
			next.IntervalDays = 3
		} else {
			next.IntervalDays = max(1, int(math.Round(float64(max(1, state.IntervalDays))*state.EaseFactor)))
		}
		next.NextDueAt = now.Add(time.Duration(next.IntervalDays) * 24 * time.Hour)
	case domain.ReviewGradeEasy:
		next.Status = domain.ReviewStatusReview
		next.ConsecutiveAgain = 0
		next.RepetitionCount++
		next.EaseFactor += 0.15
		if next.RepetitionCount == 1 {
			next.IntervalDays = 3
		} else {
			next.IntervalDays = max(2, int(math.Round(float64(max(1, state.IntervalDays))*(state.EaseFactor+0.3))))
		}
		next.NextDueAt = now.Add(time.Duration(next.IntervalDays) * 24 * time.Hour)
	default:
		return domain.ReviewState{}, errors.New("invalid grade")
	}

	log := domain.ReviewLog{
		ID:          newID("rev"),
		UserID:      userID,
		VocabItemID: vocabID,
		Grade:       grade,
		ReviewedAt:  now,
	}
	var job *domain.NotificationJob
	if !next.NextDueAt.After(now) {
		job = &domain.NotificationJob{
			ID:          newID("job"),
			UserID:      userID,
			VocabItemID: vocabID,
			ScheduledAt: now,
			Status:      "pending",
			Message:     "Time to review your vocabulary.",
		}
	}
	if err := a.store.RecordReview(context.Background(), next, log, job); err != nil {
		return domain.ReviewState{}, err
	}
	return next, nil
}

type CaptureInput struct {
	Term            string `json:"term"`
	Meaning         string `json:"meaning"`
	ExampleSentence string `json:"example_sentence"`
	Selection       string `json:"selection"`
	PageTitle       string `json:"page_title"`
	PageURL         string `json:"page_url"`
	Notes           string `json:"notes"`
}

func (a *App) CreateCapture(userID string, input CaptureInput) (DueCard, error) {
	if strings.TrimSpace(input.Term) == "" {
		return DueCard{}, errors.New("term is required")
	}
	now := a.clock.Now()
	item := domain.VocabItem{
		ID:              newID("voc"),
		UserID:          userID,
		Term:            strings.TrimSpace(input.Term),
		Kind:            domain.CardKindPhrase,
		Meaning:         strings.TrimSpace(input.Meaning),
		ExampleSentence: strings.TrimSpace(input.ExampleSentence),
		SourceText:      strings.TrimSpace(input.Selection),
		SourceURL:       strings.TrimSpace(input.PageURL),
		Notes:           strings.TrimSpace(input.Notes),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	state := domain.ReviewState{
		VocabItemID:     item.ID,
		UserID:          userID,
		Status:          domain.ReviewStatusNew,
		EaseFactor:      2.5,
		IntervalDays:    0,
		RepetitionCount: 0,
		NextDueAt:       now,
	}
	capture := domain.CaptureSource{
		ID:          newID("cap"),
		UserID:      userID,
		VocabItemID: item.ID,
		Source:      "chrome-extension",
		Selection:   input.Selection,
		PageTitle:   input.PageTitle,
		PageURL:     input.PageURL,
		CreatedAt:   now,
	}
	job := &domain.NotificationJob{
		ID:          newID("job"),
		UserID:      userID,
		VocabItemID: item.ID,
		ScheduledAt: now,
		Status:      "pending",
		Message:     "Time to review your vocabulary.",
	}
	if err := a.store.CreateCapturedVocab(context.Background(), item, state, capture, job); err != nil {
		return DueCard{}, err
	}

	return DueCard{Item: item, State: state}, nil
}

func (a *App) RegisterDevice(userID, platform, token string) (domain.DeviceToken, error) {
	if strings.TrimSpace(platform) == "" || strings.TrimSpace(token) == "" {
		return domain.DeviceToken{}, errors.New("platform and token are required")
	}
	now := a.clock.Now()
	device := domain.DeviceToken{
		ID:        newID("dev"),
		UserID:    userID,
		Platform:  platform,
		Token:     token,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return a.store.UpsertDeviceToken(context.Background(), device)
}

func (a *App) ListNotificationJobs(userID string) []domain.NotificationJob {
	jobs, err := a.store.ListNotificationJobs(context.Background(), userID)
	if err != nil {
		return nil
	}
	return jobs
}

func (a *App) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return a.store.HealthCheck(ctx)
}
