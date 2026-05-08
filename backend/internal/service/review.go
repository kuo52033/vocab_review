package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
	"vocabreview/backend/internal/service/enrichment"
	"vocabreview/backend/internal/service/intake"
	"vocabreview/backend/internal/service/scheduling"
)

var ErrEnrichmentNotConfigured = errors.New("vocab enrichment is not configured")

type VocabEnricher interface {
	Autocomplete(ctx context.Context, items []enrichment.Item) ([]enrichment.Suggestion, error)
}

type App struct {
	store    repository.AppRepository
	clock    clock.Clock
	enricher VocabEnricher
}

func NewApp(store repository.AppRepository, appClock clock.Clock) *App {
	return NewAppWithEnricher(store, appClock, nil)
}

func NewAppWithEnricher(store repository.AppRepository, appClock clock.Clock, enricher VocabEnricher) *App {
	return &App{store: store, clock: appClock, enricher: enricher}
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
	Term            string               `json:"term"`
	Kind            domain.CardKind      `json:"kind"`
	Meaning         string               `json:"meaning"`
	ExampleSentence string               `json:"example_sentence"`
	PartOfSpeech    *domain.PartOfSpeech `json:"part_of_speech"`
	SourceText      string               `json:"source_text"`
	SourceURL       string               `json:"source_url"`
	Notes           string               `json:"notes"`
}

func (a *App) CreateVocab(userID string, input CreateVocabInput) (domain.VocabItem, domain.ReviewState, error) {
	now := a.clock.Now()
	card, err := intake.NewVocabCard(userID, intake.VocabInput{
		Term:            input.Term,
		Kind:            input.Kind,
		Meaning:         input.Meaning,
		ExampleSentence: input.ExampleSentence,
		PartOfSpeech:    partOfSpeechValue(input.PartOfSpeech),
		SourceText:      input.SourceText,
		SourceURL:       input.SourceURL,
		Notes:           input.Notes,
	}, intake.IDs{
		VocabItemID:       newID("voc"),
		NotificationJobID: newID("job"),
	}, now)
	if err != nil {
		return domain.VocabItem{}, domain.ReviewState{}, err
	}
	if err := a.store.CreateVocab(context.Background(), card.Item, card.State, card.NotificationJob); err != nil {
		return domain.VocabItem{}, domain.ReviewState{}, err
	}
	return card.Item, card.State, nil
}

func (a *App) AutocompleteVocab(ctx context.Context, items []enrichment.Item) ([]enrichment.Suggestion, error) {
	if a.enricher == nil {
		return nil, ErrEnrichmentNotConfigured
	}
	return a.enricher.Autocomplete(ctx, items)
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
	if input.PartOfSpeech != nil {
		item.PartOfSpeech = *input.PartOfSpeech
	}
	item.SourceText = strings.TrimSpace(defaultString(input.SourceText, item.SourceText))
	item.SourceURL = strings.TrimSpace(defaultString(input.SourceURL, item.SourceURL))
	item.Notes = strings.TrimSpace(defaultString(input.Notes, item.Notes))
	item.UpdatedAt = a.clock.Now()
	if err := a.store.UpdateVocab(context.Background(), item); err != nil {
		return domain.VocabItem{}, err
	}
	return item, nil
}

func (a *App) ArchiveVocab(userID, id string) (domain.VocabItem, error) {
	now := a.clock.Now()
	item, err := a.store.ArchiveVocabForUser(context.Background(), userID, id, now)
	if errors.Is(err, repository.ErrNotFound) {
		return domain.VocabItem{}, errors.New("vocab not found")
	}
	if err != nil {
		return domain.VocabItem{}, err
	}
	return item, nil
}

func partOfSpeechValue(value *domain.PartOfSpeech) domain.PartOfSpeech {
	if value == nil {
		return domain.PartOfSpeechUnspecified
	}
	return *value
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

type ListVocabInput struct {
	Limit  int
	Offset int
	Query  string
	Status domain.ReviewStatus
}

type VocabPage struct {
	Items  []VocabWithState `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

func (a *App) ListVocab(userID string, input ListVocabInput) (VocabPage, error) {
	items, total, err := a.store.ListVocabByUser(context.Background(), userID, repository.ListVocabOptions{
		Pagination: repository.Pagination{Limit: input.Limit, Offset: input.Offset},
		Query:      strings.TrimSpace(input.Query),
		Status:     input.Status,
	})
	if err != nil {
		return VocabPage{}, err
	}
	result := make([]VocabWithState, 0, len(items))
	for _, item := range items {
		result = append(result, VocabWithState{Item: item.Item, State: item.State})
	}
	return VocabPage{Items: result, Total: total, Limit: input.Limit, Offset: input.Offset}, nil
}

type DueCard struct {
	Item  domain.VocabItem   `json:"item"`
	State domain.ReviewState `json:"state"`
}

type ReviewHistoryEntry struct {
	Log   domain.ReviewLog   `json:"log"`
	Item  domain.VocabItem   `json:"item"`
	State domain.ReviewState `json:"state"`
}

type PageInput struct {
	Limit  int
	Offset int
}

type ReviewHistoryPage struct {
	Items  []ReviewHistoryEntry `json:"items"`
	Total  int                  `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

type ReviewStats struct {
	ReviewedToday int `json:"reviewed_today"`
	Reviewed7Days int `json:"reviewed_7_days"`
	ActiveCards   int `json:"active_cards"`
	DueNow        int `json:"due_now"`
	ArchivedCards int `json:"archived_cards"`
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

	now := a.clock.Now()
	next, err := scheduling.ScheduleReview(state, grade, now)
	if err != nil {
		return domain.ReviewState{}, err
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

func (a *App) ReviewHistory(userID string, input PageInput) (ReviewHistoryPage, error) {
	entries, total, err := a.store.ListReviewHistory(context.Background(), userID, repository.Pagination{Limit: input.Limit, Offset: input.Offset})
	if err != nil {
		return ReviewHistoryPage{}, err
	}
	result := make([]ReviewHistoryEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, ReviewHistoryEntry{Log: entry.Log, Item: entry.Item, State: entry.State})
	}
	return ReviewHistoryPage{Items: result, Total: total, Limit: input.Limit, Offset: input.Offset}, nil
}

func (a *App) ReviewStats(userID string) (ReviewStats, error) {
	stats, err := a.store.GetReviewStats(context.Background(), userID, a.clock.Now())
	if err != nil {
		return ReviewStats{}, err
	}
	return ReviewStats{
		ReviewedToday: stats.ReviewedToday,
		Reviewed7Days: stats.Reviewed7Days,
		ActiveCards:   stats.ActiveCards,
		DueNow:        stats.DueNow,
		ArchivedCards: stats.ArchivedCards,
	}, nil
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
	now := a.clock.Now()
	card, err := intake.NewCapturedCard(userID, intake.CaptureInput{
		Term:            input.Term,
		Meaning:         input.Meaning,
		ExampleSentence: input.ExampleSentence,
		Selection:       input.Selection,
		PageTitle:       input.PageTitle,
		PageURL:         input.PageURL,
		Notes:           input.Notes,
	}, intake.IDs{
		VocabItemID:       newID("voc"),
		CaptureSourceID:   newID("cap"),
		NotificationJobID: newID("job"),
	}, now)
	if err != nil {
		return DueCard{}, err
	}
	if err := a.store.CreateCapturedVocab(context.Background(), card.Item, card.State, card.Capture, card.NotificationJob); err != nil {
		return DueCard{}, err
	}

	return DueCard{Item: card.Item, State: card.State}, nil
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
