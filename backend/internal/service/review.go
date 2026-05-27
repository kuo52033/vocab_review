package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
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

type MagicLinkSender interface {
	SendMagicLink(ctx context.Context, email, verificationURL, token string, expiresAt time.Time) error
}

type App struct {
	store           repository.AppRepository
	clock           clock.Clock
	enricher        VocabEnricher
	authConfig      AuthConfig
	debugEmails     map[string]struct{}
	magicLinkSender MagicLinkSender
}

type AuthConfig struct {
	Environment      string
	TokenHashSecret  string
	PublicWebBaseURL string
	DebugEmails      []string
}

func NewApp(store repository.AppRepository, appClock clock.Clock) *App {
	return NewAppWithEnricher(store, appClock, nil)
}

func NewAppWithEnricher(store repository.AppRepository, appClock clock.Clock, enricher VocabEnricher) *App {
	return NewAppWithConfig(store, appClock, enricher, AuthConfig{
		Environment:     "development",
		TokenHashSecret: "development-token-hash-secret",
	}, nil)
}

func NewAppWithConfig(store repository.AppRepository, appClock clock.Clock, enricher VocabEnricher, config AuthConfig, sender MagicLinkSender) *App {
	config.Environment = strings.ToLower(strings.TrimSpace(config.Environment))
	if config.Environment == "" {
		config.Environment = "production"
	}
	if config.Environment == "development" && config.TokenHashSecret == "" {
		config.TokenHashSecret = "development-token-hash-secret"
	}
	debugEmails := make(map[string]struct{}, len(config.DebugEmails))
	for _, email := range config.DebugEmails {
		normalized := normalizeEmail(email)
		if normalized != "" {
			debugEmails[normalized] = struct{}{}
		}
	}
	return &App{store: store, clock: appClock, enricher: enricher, authConfig: config, debugEmails: debugEmails, magicLinkSender: sender}
}

type AuthResult struct {
	User        domain.User `json:"user"`
	Session     AuthSession `json:"session"`
	RedirectURL string      `json:"redirect_url"`
}

type AuthSession struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type MagicLinkResponse struct {
	Message         string `json:"message"`
	Token           string `json:"token,omitempty"`
	VerificationURL string `json:"verification_url,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
}

func (a *App) RequestMagicLink(ctx context.Context, email, baseURL, client string) (MagicLinkResponse, error) {
	email = normalizeEmail(email)
	if !validEmail(email) {
		return MagicLinkResponse{}, errors.New("valid email is required")
	}

	response := MagicLinkResponse{Message: "Check your email for the sign-in link."}
	isDevelopment := a.authConfig.Environment == "development"
	isDebugEmail := a.isDebugEmail(email)
	if !isDevelopment {
		if _, ok, err := a.store.GetUserByEmail(ctx, email); err != nil {
			return MagicLinkResponse{}, err
		} else if !ok {
			return response, nil
		}
	}

	rawToken := newID("ml")
	token := domain.MagicLinkToken{
		TokenHash: a.hashToken(rawToken),
		Email:     email,
		ExpiresAt: a.clock.Now().Add(15 * time.Minute),
	}
	if err := a.store.PutMagicLink(ctx, token); err != nil {
		return MagicLinkResponse{}, err
	}

	verificationURL := a.verificationURL(baseURL, rawToken, client)
	if isDevelopment || isDebugEmail {
		response.Token = rawToken
		response.VerificationURL = verificationURL
		response.ExpiresAt = token.ExpiresAt.Format(time.RFC3339)
	}
	if !isDevelopment && a.magicLinkSender != nil {
		if err := a.magicLinkSender.SendMagicLink(ctx, email, verificationURL, rawToken, token.ExpiresAt); err != nil {
			return response, nil
		}
	}

	return response, nil
}

func (a *App) VerifyMagicLink(ctx context.Context, token string) (AuthResult, error) {
	now := a.clock.Now()
	token = strings.TrimSpace(token)
	if token == "" {
		return AuthResult{}, errors.New("invalid token")
	}
	sessionToken := newID("sess")
	newUser := domain.User{
		ID:        newID("usr"),
		CreatedAt: now,
	}
	newSession := domain.Session{
		TokenHash: a.hashToken(sessionToken),
		CreatedAt: now,
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}
	user, session, err := a.store.ConsumeMagicLink(ctx, a.hashToken(token), now, newUser, newSession)
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
		Session:     AuthSession{Token: sessionToken, ExpiresAt: session.ExpiresAt},
		RedirectURL: "/auth/success?session_token=" + sessionToken,
	}, nil
}

func (a *App) Session(ctx context.Context, token string) (domain.Session, domain.User, error) {
	session, user, ok, err := a.store.GetSessionUser(ctx, a.hashToken(strings.TrimSpace(token)))
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

func (a *App) hashToken(token string) string {
	mac := hmac.New(sha256.New, []byte(a.authConfig.TokenHashSecret))
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *App) verificationURL(baseURL, token, client string) string {
	webBaseURL := a.authConfig.PublicWebBaseURL
	if a.authConfig.Environment == "development" && strings.TrimSpace(baseURL) != "" {
		webBaseURL = baseURL
	}
	if strings.TrimSpace(webBaseURL) == "" {
		webBaseURL = "http://localhost:8080"
	}
	return strings.TrimRight(webBaseURL, "/") + "?token=" + token
}

func (a *App) isDebugEmail(email string) bool {
	_, ok := a.debugEmails[email]
	return ok
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func validEmail(email string) bool {
	if email == "" {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && strings.EqualFold(address.Address, email)
}

type CreateVocabInput struct {
	Term            string               `json:"term"`
	Meaning         string               `json:"meaning"`
	Chinese         string               `json:"chinese"`
	ExampleSentence string               `json:"example_sentence"`
	PartOfSpeech    *domain.PartOfSpeech `json:"part_of_speech"`
	SourceText      string               `json:"source_text"`
	SourceURL       string               `json:"source_url"`
	Notes           string               `json:"notes"`
}

type CreateVocabResult struct {
	Item             domain.VocabItem   `json:"item"`
	State            domain.ReviewState `json:"state"`
	Created          bool               `json:"created"`
	SkippedDuplicate bool               `json:"skipped_duplicate"`
}

func (a *App) CreateVocab(ctx context.Context, userID string, input CreateVocabInput) (CreateVocabResult, error) {
	term := strings.TrimSpace(input.Term)
	if term == "" {
		return CreateVocabResult{}, errors.New("term is required")
	}
	existing, ok, err := a.store.GetActiveVocabByTerm(ctx, userID, term)
	if err != nil {
		return CreateVocabResult{}, err
	}
	if ok {
		return CreateVocabResult{
			Item:             existing.Item,
			State:            existing.State,
			Created:          false,
			SkippedDuplicate: true,
		}, nil
	}

	now := a.clock.Now()
	card, err := intake.NewVocabCard(userID, vocabInput(input), intake.IDs{
		VocabItemID:       newID("voc"),
		NotificationJobID: newID("job"),
	}, now)
	if err != nil {
		return CreateVocabResult{}, err
	}
	if err := a.store.CreateVocab(ctx, card.Item, card.State, card.NotificationJob); err != nil {
		return CreateVocabResult{}, err
	}
	return CreateVocabResult{Item: card.Item, State: card.State, Created: true}, nil
}

func (a *App) AutocompleteVocab(ctx context.Context, items []enrichment.Item) ([]enrichment.Suggestion, error) {
	if a.enricher == nil {
		return nil, ErrEnrichmentNotConfigured
	}
	return a.enricher.Autocomplete(ctx, items)
}

func (a *App) UpdateVocab(ctx context.Context, userID, id string, input CreateVocabInput) (domain.VocabItem, error) {
	item, ok, err := a.store.GetVocab(ctx, id)
	if err != nil {
		return domain.VocabItem{}, err
	}
	if !ok || item.UserID != userID {
		return domain.VocabItem{}, errors.New("vocab not found")
	}
	item.Term = updatedString(input.Term, item.Term)
	item.Meaning = updatedString(input.Meaning, item.Meaning)
	item.Chinese = updatedString(input.Chinese, item.Chinese)
	item.ExampleSentence = updatedString(input.ExampleSentence, item.ExampleSentence)
	if input.PartOfSpeech != nil {
		item.PartOfSpeech = *input.PartOfSpeech
	}
	item.SourceText = updatedString(input.SourceText, item.SourceText)
	item.SourceURL = updatedString(input.SourceURL, item.SourceURL)
	item.Notes = updatedString(input.Notes, item.Notes)
	item.UpdatedAt = a.clock.Now()
	if err := a.store.UpdateVocab(ctx, item); err != nil {
		return domain.VocabItem{}, err
	}
	return item, nil
}

func (a *App) ArchiveVocab(ctx context.Context, userID, id string) (domain.VocabItem, error) {
	now := a.clock.Now()
	item, err := a.store.ArchiveVocabForUser(ctx, userID, id, now)
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

func vocabInput(input CreateVocabInput) intake.VocabInput {
	return intake.VocabInput{
		Term:            input.Term,
		Meaning:         input.Meaning,
		Chinese:         input.Chinese,
		ExampleSentence: input.ExampleSentence,
		PartOfSpeech:    partOfSpeechValue(input.PartOfSpeech),
		SourceText:      input.SourceText,
		SourceURL:       input.SourceURL,
		Notes:           input.Notes,
	}
}

func updatedString(next, current string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return strings.TrimSpace(current)
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

func (a *App) ListVocab(ctx context.Context, userID string, input ListVocabInput) (VocabPage, error) {
	items, total, err := a.store.ListVocabByUser(ctx, userID, repository.ListVocabOptions{
		Pagination: repository.Pagination{Limit: input.Limit, Offset: input.Offset},
		Query:      strings.TrimSpace(input.Query),
		Status:     input.Status,
	})
	if err != nil {
		return VocabPage{}, err
	}
	return VocabPage{
		Items:  vocabWithStates(items),
		Total:  total,
		Limit:  input.Limit,
		Offset: input.Offset,
	}, nil
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

func (a *App) DueCards(ctx context.Context, userID string) ([]DueCard, error) {
	states, err := a.store.ListDueVocab(ctx, userID, a.clock.Now())
	if err != nil {
		return nil, err
	}
	return dueCards(states), nil
}

func (a *App) GradeReview(ctx context.Context, userID, vocabID string, grade domain.ReviewGrade) (domain.ReviewState, error) {
	state, ok, err := a.store.GetReviewState(ctx, vocabID)
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
		job = reviewReminderJob(newID("job"), userID, vocabID, now)
	}
	if err := a.store.RecordReview(ctx, next, log, job); err != nil {
		return domain.ReviewState{}, err
	}
	return next, nil
}

func (a *App) ReviewHistory(ctx context.Context, userID string, input PageInput) (ReviewHistoryPage, error) {
	entries, total, err := a.store.ListReviewHistory(ctx, userID, repository.Pagination{Limit: input.Limit, Offset: input.Offset})
	if err != nil {
		return ReviewHistoryPage{}, err
	}
	return ReviewHistoryPage{
		Items:  reviewHistoryEntries(entries),
		Total:  total,
		Limit:  input.Limit,
		Offset: input.Offset,
	}, nil
}

func (a *App) ReviewStats(ctx context.Context, userID string) (ReviewStats, error) {
	stats, err := a.store.GetReviewStats(ctx, userID, a.clock.Now())
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
	Chinese         string `json:"chinese"`
	ExampleSentence string `json:"example_sentence"`
	PartOfSpeech    string `json:"part_of_speech"`
	Selection       string `json:"selection"`
	PageTitle       string `json:"page_title"`
	PageURL         string `json:"page_url"`
	Notes           string `json:"notes"`
}

func (a *App) CreateCapture(ctx context.Context, userID string, input CaptureInput) (DueCard, error) {
	now := a.clock.Now()
	card, err := intake.NewCapturedCard(userID, intake.CaptureInput{
		Term:            input.Term,
		Meaning:         input.Meaning,
		Chinese:         input.Chinese,
		ExampleSentence: input.ExampleSentence,
		PartOfSpeech:    domain.PartOfSpeech(strings.TrimSpace(input.PartOfSpeech)),
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
	if err := a.store.CreateCapturedVocab(ctx, card.Item, card.State, card.Capture, card.NotificationJob); err != nil {
		return DueCard{}, err
	}

	return DueCard{Item: card.Item, State: card.State}, nil
}

func (a *App) RegisterDevice(ctx context.Context, userID, platform, token string) (domain.DeviceToken, error) {
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
	return a.store.UpsertDeviceToken(ctx, device)
}

func (a *App) ListNotificationJobs(ctx context.Context, userID string) ([]domain.NotificationJob, error) {
	return a.store.ListNotificationJobs(ctx, userID)
}

func (a *App) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return a.store.HealthCheck(ctx)
}

func vocabWithStates(items []repository.VocabWithState) []VocabWithState {
	result := make([]VocabWithState, 0, len(items))
	for _, item := range items {
		result = append(result, VocabWithState{Item: item.Item, State: item.State})
	}
	return result
}

func dueCards(items []repository.VocabWithState) []DueCard {
	result := make([]DueCard, 0, len(items))
	for _, item := range items {
		result = append(result, DueCard{Item: item.Item, State: item.State})
	}
	return result
}

func reviewHistoryEntries(entries []repository.ReviewHistoryEntry) []ReviewHistoryEntry {
	result := make([]ReviewHistoryEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, ReviewHistoryEntry{Log: entry.Log, Item: entry.Item, State: entry.State})
	}
	return result
}

func reviewReminderJob(id, userID, vocabID string, scheduledAt time.Time) *domain.NotificationJob {
	return &domain.NotificationJob{
		ID:          id,
		UserID:      userID,
		VocabItemID: vocabID,
		ScheduledAt: scheduledAt,
		Status:      "pending",
		Message:     "Time to review your vocabulary.",
	}
}
