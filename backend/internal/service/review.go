package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
	"vocabreview/backend/internal/service/audios"
	"vocabreview/backend/internal/service/enrichment"
	"vocabreview/backend/internal/service/intake"
	"vocabreview/backend/internal/service/scheduling"
)

var ErrEnrichmentNotConfigured = errors.New("vocab enrichment is not configured")
var (
	ErrVocabAudioNotFound       = errors.New("vocab audio not found")
	ErrVocabAudioNotReady       = errors.New("vocab audio is not ready")
	ErrVocabAudioURLUnavailable = errors.New("vocab audio url is unavailable")
)

type VocabEnricher interface {
	Autocomplete(ctx context.Context, items []enrichment.Item) ([]enrichment.Suggestion, error)
}

type VocabAudioURLSigner interface {
	SignVocabAudioURL(ctx context.Context, storageKey string) (string, error)
}

type App struct {
	store           repository.AppRepository
	clock           clock.Clock
	enricher        VocabEnricher
	authConfig      AuthConfig
	audioConfig     VocabAudioConfig
	audioURLSigner  VocabAudioURLSigner
	debugEmails     map[string]struct{}
	magicLinkSender MagicLinkSender
}

type VocabAudioConfig struct {
	Enabled       bool
	Provider      string
	Model         string
	Voice         string
	Speed         float64
	OutputFormat  string
	PublicBaseURL string
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
	return NewAppWithVocabAudioConfig(store, appClock, enricher, config, sender, VocabAudioConfig{})
}

func NewAppWithVocabAudioConfig(store repository.AppRepository, appClock clock.Clock, enricher VocabEnricher, config AuthConfig, sender MagicLinkSender, audioConfig VocabAudioConfig) *App {
	return NewAppWithVocabAudioConfigAndSigner(store, appClock, enricher, config, sender, audioConfig, nil)
}

func NewAppWithVocabAudioConfigAndSigner(store repository.AppRepository, appClock clock.Clock, enricher VocabEnricher, config AuthConfig, sender MagicLinkSender, audioConfig VocabAudioConfig, signer VocabAudioURLSigner) *App {
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
	audioConfig.Provider = strings.TrimSpace(audioConfig.Provider)
	audioConfig.Model = strings.TrimSpace(audioConfig.Model)
	audioConfig.Voice = strings.TrimSpace(audioConfig.Voice)
	audioConfig.OutputFormat = strings.TrimSpace(audioConfig.OutputFormat)
	if audioConfig.Speed == 0 {
		audioConfig.Speed = 1
	}
	return &App{store: store, clock: appClock, enricher: enricher, authConfig: config, audioConfig: audioConfig, audioURLSigner: signer, debugEmails: debugEmails, magicLinkSender: sender}
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
		existing.Item = a.decorateAudio(existing.Item)
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
	audioJob, err := a.prepareAudioJob(ctx, &card.Item, term, now)
	if err != nil {
		return CreateVocabResult{}, err
	}
	if err := a.store.CreateVocab(ctx, card.Item, card.State, card.NotificationJob, audioJob); err != nil {
		return CreateVocabResult{}, err
	}
	card.Item = a.decorateAudio(card.Item)
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
	previousTerm := audios.NormalizeInput(item.Term)
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
	var audioJob *domain.VocabAudioJob
	if audios.NormalizeInput(item.Term) != previousTerm {
		audioJob, err = a.prepareAudioJob(ctx, &item, item.Term, item.UpdatedAt)
		if err != nil {
			return domain.VocabItem{}, err
		}
	}
	if err := a.store.UpdateVocab(ctx, item, audioJob); err != nil {
		return domain.VocabItem{}, err
	}
	return a.decorateAudio(item), nil
}

func (a *App) VocabAudioURL(ctx context.Context, userID, vocabID string) (string, error) {
	item, ok, err := a.store.GetVocab(ctx, vocabID)
	if err != nil {
		return "", err
	}
	if !ok || item.UserID != userID {
		return "", ErrVocabAudioNotFound
	}
	item = a.decorateAudio(item)
	if item.Audio == nil || item.Audio.Status == "unavailable" {
		return "", ErrVocabAudioNotFound
	}
	if item.Audio.Status != "ready" {
		return "", ErrVocabAudioNotReady
	}
	if item.Audio.URL != "" {
		return item.Audio.URL, nil
	}
	if item.Audio.StorageKey == "" || a.audioURLSigner == nil {
		return "", ErrVocabAudioURLUnavailable
	}
	return a.audioURLSigner.SignVocabAudioURL(ctx, item.Audio.StorageKey)
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
		Items:  a.vocabWithStates(items),
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
	return a.dueCards(states), nil
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
		Items:  a.reviewHistoryEntries(entries),
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
	term := strings.TrimSpace(input.Term)
	if term == "" {
		return DueCard{}, errors.New("term is required")
	}
	existing, ok, err := a.store.GetActiveVocabByTerm(ctx, userID, term)
	if err != nil {
		return DueCard{}, err
	}
	if ok {
		existing.Item = a.decorateAudio(existing.Item)
		return DueCard{Item: existing.Item, State: existing.State}, nil
	}

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
	audioJob, err := a.prepareAudioJob(ctx, &card.Item, term, now)
	if err != nil {
		return DueCard{}, err
	}
	if err := a.store.CreateCapturedVocab(ctx, card.Item, card.State, card.Capture, card.NotificationJob, audioJob); err != nil {
		return DueCard{}, err
	}

	card.Item = a.decorateAudio(card.Item)
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

func (a *App) vocabWithStates(items []repository.VocabWithState) []VocabWithState {
	result := make([]VocabWithState, 0, len(items))
	for _, item := range items {
		item.Item = a.decorateAudio(item.Item)
		result = append(result, VocabWithState{Item: item.Item, State: item.State})
	}
	return result
}

func (a *App) dueCards(items []repository.VocabWithState) []DueCard {
	result := make([]DueCard, 0, len(items))
	for _, item := range items {
		item.Item = a.decorateAudio(item.Item)
		result = append(result, DueCard{Item: item.Item, State: item.State})
	}
	return result
}

func (a *App) reviewHistoryEntries(entries []repository.ReviewHistoryEntry) []ReviewHistoryEntry {
	result := make([]ReviewHistoryEntry, 0, len(entries))
	for _, entry := range entries {
		entry.Item = a.decorateAudio(entry.Item)
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

func (a *App) prepareAudioJob(ctx context.Context, item *domain.VocabItem, term string, now time.Time) (*domain.VocabAudioJob, error) {
	item.AudioID = ""
	item.Audio = nil
	if !a.audioConfig.Enabled {
		return nil, nil
	}
	inputText := audios.NormalizeInput(term)
	if inputText == "" {
		return nil, nil
	}
	inputHash := audios.InputHash(audioGenerationConfig(a.audioConfig), inputText)
	audio, ok, err := a.store.GetReadyVocabAudio(ctx, a.audioConfig.Provider, a.audioConfig.Model, a.audioConfig.Voice, a.audioConfig.Speed, a.audioConfig.OutputFormat, inputHash)
	if err != nil {
		return nil, err
	}
	if ok {
		item.AudioID = audio.ID
		item.Audio = &audio
		return nil, nil
	}
	item.Audio = &domain.VocabAudio{
		Provider:     a.audioConfig.Provider,
		Model:        a.audioConfig.Model,
		Voice:        a.audioConfig.Voice,
		Speed:        a.audioConfig.Speed,
		OutputFormat: a.audioConfig.OutputFormat,
		Status:       "pending",
	}
	return &domain.VocabAudioJob{
		ID:            newID("audjob"),
		VocabItemID:   item.ID,
		Provider:      a.audioConfig.Provider,
		Model:         a.audioConfig.Model,
		Voice:         a.audioConfig.Voice,
		Speed:         a.audioConfig.Speed,
		OutputFormat:  a.audioConfig.OutputFormat,
		InputText:     inputText,
		InputHash:     inputHash,
		Status:        "pending",
		MaxAttempts:   3,
		NextAttemptAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (a *App) decorateAudio(item domain.VocabItem) domain.VocabItem {
	if item.Audio == nil {
		item.Audio = &domain.VocabAudio{Status: "unavailable"}
		return item
	}
	if item.Audio.StorageKey != "" && strings.TrimSpace(a.audioConfig.PublicBaseURL) != "" {
		item.Audio.URL = strings.TrimRight(a.audioConfig.PublicBaseURL, "/") + "/" + strings.TrimLeft(item.Audio.StorageKey, "/")
	}
	return item
}

func audioGenerationConfig(config VocabAudioConfig) audios.GenerationConfig {
	return audios.GenerationConfig{
		Provider:     config.Provider,
		Model:        config.Model,
		Voice:        config.Voice,
		Speed:        config.Speed,
		OutputFormat: config.OutputFormat,
	}.Normalized()
}
