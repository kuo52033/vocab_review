package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
	"vocabreview/backend/internal/service/enrichment"
)

type stubClock struct {
	now time.Time
}

func (s stubClock) Now() time.Time { return s.now }

type fakeRepository struct {
	users            map[string]domain.User
	usersByEmail     map[string]string
	sessions         map[string]domain.Session
	magicLinks       map[string]domain.MagicLinkToken
	vocab            map[string]domain.VocabItem
	reviewStates     map[string]domain.ReviewState
	reviewLogs       map[string]domain.ReviewLog
	captures         map[string]domain.CaptureSource
	deviceTokens     map[string]domain.DeviceToken
	notificationJobs map[string]domain.NotificationJob
	seenContextValue any
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		users:            map[string]domain.User{},
		usersByEmail:     map[string]string{},
		sessions:         map[string]domain.Session{},
		magicLinks:       map[string]domain.MagicLinkToken{},
		vocab:            map[string]domain.VocabItem{},
		reviewStates:     map[string]domain.ReviewState{},
		reviewLogs:       map[string]domain.ReviewLog{},
		captures:         map[string]domain.CaptureSource{},
		deviceTokens:     map[string]domain.DeviceToken{},
		notificationJobs: map[string]domain.NotificationJob{},
	}
}

func (f *fakeRepository) HealthCheck(context.Context) error { return nil }

func (f *fakeRepository) PutMagicLink(_ context.Context, token domain.MagicLinkToken) error {
	for tokenHash, existing := range f.magicLinks {
		if existing.Email == token.Email {
			delete(f.magicLinks, tokenHash)
		}
	}
	f.magicLinks[token.TokenHash] = token
	return nil
}

func (f *fakeRepository) GetUserByEmail(_ context.Context, email string) (domain.User, bool, error) {
	userID, ok := f.usersByEmail[email]
	if !ok {
		return domain.User{}, false, nil
	}
	return f.users[userID], true, nil
}

func (f *fakeRepository) ConsumeMagicLink(_ context.Context, tokenHash string, now time.Time, newUser domain.User, newSession domain.Session) (domain.User, domain.Session, error) {
	link, ok := f.magicLinks[tokenHash]
	if !ok {
		return domain.User{}, domain.Session{}, repository.ErrNotFound
	}
	if now.After(link.ExpiresAt) {
		return domain.User{}, domain.Session{}, repository.ErrExpired
	}

	var user domain.User
	if userID, exists := f.usersByEmail[link.Email]; exists {
		user = f.users[userID]
	} else {
		newUser.Email = link.Email
		user = newUser
		f.users[user.ID] = user
		f.usersByEmail[user.Email] = user.ID
	}

	newSession.UserID = user.ID
	f.sessions[newSession.TokenHash] = newSession
	delete(f.magicLinks, tokenHash)
	return user, newSession, nil
}

func (f *fakeRepository) GetSessionUser(_ context.Context, tokenHash string) (domain.Session, domain.User, bool, error) {
	session, ok := f.sessions[tokenHash]
	if !ok {
		return domain.Session{}, domain.User{}, false, nil
	}
	user, ok := f.users[session.UserID]
	if !ok {
		return domain.Session{}, domain.User{}, false, errors.New("user not found")
	}
	return session, user, true, nil
}

func (f *fakeRepository) CreateVocab(ctx context.Context, item domain.VocabItem, state domain.ReviewState, job *domain.NotificationJob) error {
	f.seenContextValue = ctx.Value(testContextKey{})
	f.vocab[item.ID] = item
	f.reviewStates[state.VocabItemID] = state
	if job != nil {
		f.notificationJobs[job.ID] = *job
	}
	return nil
}

func (f *fakeRepository) CreateCapturedVocab(ctx context.Context, item domain.VocabItem, state domain.ReviewState, capture domain.CaptureSource, job *domain.NotificationJob) error {
	if err := f.CreateVocab(ctx, item, state, job); err != nil {
		return err
	}
	f.captures[capture.ID] = capture
	return nil
}

func (f *fakeRepository) GetVocab(_ context.Context, id string) (domain.VocabItem, bool, error) {
	item, ok := f.vocab[id]
	return item, ok, nil
}

func (f *fakeRepository) UpdateVocab(_ context.Context, item domain.VocabItem) error {
	f.vocab[item.ID] = item
	return nil
}

func (f *fakeRepository) ArchiveVocabForUser(_ context.Context, userID string, vocabID string, archivedAt time.Time) (domain.VocabItem, error) {
	item, ok := f.vocab[vocabID]
	if !ok || item.UserID != userID {
		return domain.VocabItem{}, repository.ErrNotFound
	}
	item.ArchivedAt = &archivedAt
	item.UpdatedAt = archivedAt
	f.vocab[item.ID] = item
	return item, nil
}

func (f *fakeRepository) GetActiveVocabByTerm(_ context.Context, userID string, term string) (repository.VocabWithState, bool, error) {
	normalizedTerm := strings.ToLower(strings.TrimSpace(term))
	for _, item := range f.vocab {
		if item.UserID != userID || item.ArchivedAt != nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(item.Term)) != normalizedTerm {
			continue
		}
		return repository.VocabWithState{Item: item, State: f.reviewStates[item.ID]}, true, nil
	}
	return repository.VocabWithState{}, false, nil
}

func (f *fakeRepository) ListVocabByUser(_ context.Context, userID string, options repository.ListVocabOptions) ([]repository.VocabWithState, int, error) {
	items := make([]repository.VocabWithState, 0)
	for _, item := range f.vocab {
		if item.UserID != userID || item.ArchivedAt != nil {
			continue
		}
		state := f.reviewStates[item.ID]
		if options.Status != "" && state.Status != options.Status {
			continue
		}
		if options.Query != "" {
			haystack := strings.ToLower(item.Term + " " + item.Meaning + " " + item.ExampleSentence + " " + item.Notes)
			if !strings.Contains(haystack, strings.ToLower(options.Query)) {
				continue
			}
		}
		items = append(items, repository.VocabWithState{Item: item, State: state})
	}
	total := len(items)
	if options.Offset > len(items) {
		return []repository.VocabWithState{}, total, nil
	}
	if options.Offset > 0 {
		items = items[options.Offset:]
	}
	if options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return items, total, nil
}

func (f *fakeRepository) ListDueVocab(_ context.Context, userID string, now time.Time) ([]repository.VocabWithState, error) {
	items := make([]repository.VocabWithState, 0)
	for _, state := range f.reviewStates {
		if state.UserID != userID || state.NextDueAt.After(now) {
			continue
		}
		item := f.vocab[state.VocabItemID]
		if item.ArchivedAt != nil {
			continue
		}
		items = append(items, repository.VocabWithState{Item: item, State: state})
	}
	return items, nil
}

func (f *fakeRepository) GetReviewState(_ context.Context, vocabID string) (domain.ReviewState, bool, error) {
	state, ok := f.reviewStates[vocabID]
	return state, ok, nil
}

func (f *fakeRepository) RecordReview(_ context.Context, state domain.ReviewState, log domain.ReviewLog, job *domain.NotificationJob) error {
	f.reviewStates[state.VocabItemID] = state
	f.reviewLogs[log.ID] = log
	if job != nil {
		for _, existing := range f.notificationJobs {
			if existing.UserID == job.UserID && existing.VocabItemID == job.VocabItemID && existing.Status == "pending" {
				return nil
			}
		}
		f.notificationJobs[job.ID] = *job
	}
	return nil
}

func (f *fakeRepository) ListReviewHistory(_ context.Context, userID string, pagination repository.Pagination) ([]repository.ReviewHistoryEntry, int, error) {
	entries := make([]repository.ReviewHistoryEntry, 0)
	for _, log := range f.reviewLogs {
		if log.UserID != userID {
			continue
		}
		entries = append(entries, repository.ReviewHistoryEntry{
			Log:   log,
			Item:  f.vocab[log.VocabItemID],
			State: f.reviewStates[log.VocabItemID],
		})
	}
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Log.ReviewedAt.After(entries[i].Log.ReviewedAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	total := len(entries)
	if pagination.Offset > len(entries) {
		return []repository.ReviewHistoryEntry{}, total, nil
	}
	if pagination.Offset > 0 {
		entries = entries[pagination.Offset:]
	}
	if pagination.Limit > 0 && len(entries) > pagination.Limit {
		entries = entries[:pagination.Limit]
	}
	return entries, total, nil
}

func (f *fakeRepository) GetReviewStats(_ context.Context, userID string, now time.Time) (repository.ReviewStats, error) {
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	sevenDaysAgo := now.AddDate(0, 0, -7)
	var stats repository.ReviewStats
	for _, log := range f.reviewLogs {
		if log.UserID != userID {
			continue
		}
		if !log.ReviewedAt.Before(startOfToday) {
			stats.ReviewedToday++
		}
		if !log.ReviewedAt.Before(sevenDaysAgo) {
			stats.Reviewed7Days++
		}
	}
	for _, item := range f.vocab {
		if item.UserID != userID {
			continue
		}
		if item.ArchivedAt != nil {
			stats.ArchivedCards++
			continue
		}
		stats.ActiveCards++
		state := f.reviewStates[item.ID]
		if !state.NextDueAt.After(now) {
			stats.DueNow++
		}
	}
	return stats, nil
}

func (f *fakeRepository) UpsertDeviceToken(_ context.Context, token domain.DeviceToken) (domain.DeviceToken, error) {
	for id, existing := range f.deviceTokens {
		if existing.UserID == token.UserID && existing.Token == token.Token {
			token.ID = id
			token.CreatedAt = existing.CreatedAt
		}
	}
	f.deviceTokens[token.ID] = token
	return token, nil
}

func (f *fakeRepository) ListNotificationJobs(_ context.Context, userID string) ([]domain.NotificationJob, error) {
	jobs := make([]domain.NotificationJob, 0)
	for _, job := range f.notificationJobs {
		if job.UserID == userID {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func (f *fakeRepository) ClaimDueNotificationJobs(_ context.Context, now time.Time, limit int) ([]domain.NotificationJob, error) {
	jobs := make([]domain.NotificationJob, 0)
	for _, job := range f.notificationJobs {
		if job.Status == "pending" && !job.ScheduledAt.After(now) {
			jobs = append(jobs, job)
		}
	}
	if limit > 0 && len(jobs) > limit {
		jobs = jobs[:limit]
	}
	return jobs, nil
}

func (f *fakeRepository) ListDeviceTokensForUser(_ context.Context, userID string) ([]domain.DeviceToken, error) {
	tokens := make([]domain.DeviceToken, 0)
	for _, token := range f.deviceTokens {
		if token.UserID == userID {
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}

func (f *fakeRepository) MarkNotificationSent(_ context.Context, jobID string, sentAt time.Time) error {
	job := f.notificationJobs[jobID]
	job.Status = "sent"
	job.SentAt = &sentAt
	f.notificationJobs[jobID] = job
	return nil
}

func (f *fakeRepository) MarkNotificationFailed(_ context.Context, jobID string) error {
	job := f.notificationJobs[jobID]
	job.Status = "failed"
	f.notificationJobs[jobID] = job
	return nil
}

func (f *fakeRepository) MarkNotificationPending(_ context.Context, jobID string) error {
	job := f.notificationJobs[jobID]
	job.Status = "pending"
	f.notificationJobs[jobID] = job
	return nil
}

func newTestApp() *App {
	return NewApp(newFakeRepository(), stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)})
}

type sentMagicLink struct {
	email           string
	verificationURL string
	token           string
	expiresAt       time.Time
}

type fakeMagicLinkSender struct {
	sends []sentMagicLink
	err   error
}

func (f *fakeMagicLinkSender) SendMagicLink(_ context.Context, email, verificationURL, token string, expiresAt time.Time) error {
	f.sends = append(f.sends, sentMagicLink{email: email, verificationURL: verificationURL, token: token, expiresAt: expiresAt})
	return f.err
}

func TestMagicLinkTokensAreHashedAtRest(t *testing.T) {
	repo := newFakeRepository()
	app := NewApp(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)})

	link, err := app.RequestMagicLink(context.Background(), "test@example.com", "http://localhost:8080", "")
	if err != nil {
		t.Fatalf("request link: %v", err)
	}
	if _, ok := repo.magicLinks[link.Token]; ok {
		t.Fatal("raw magic link token was stored")
	}
	tokenHash := app.hashToken(link.Token)
	stored, ok := repo.magicLinks[tokenHash]
	if !ok {
		t.Fatal("hashed magic link token was not stored")
	}
	if stored.TokenHash == link.Token {
		t.Fatal("stored magic link token hash equals raw token")
	}

	auth, err := app.VerifyMagicLink(context.Background(), link.Token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, ok := repo.sessions[auth.Session.Token]; ok {
		t.Fatal("raw session token was stored")
	}
	if _, ok := repo.sessions[app.hashToken(auth.Session.Token)]; !ok {
		t.Fatal("hashed session token was not stored")
	}
}

func TestProductionMagicLinkForUnknownEmailIsGeneric(t *testing.T) {
	repo := newFakeRepository()
	sender := &fakeMagicLinkSender{}
	app := NewAppWithConfig(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)}, nil, AuthConfig{
		Environment:      "production",
		TokenHashSecret:  "secret",
		PublicWebBaseURL: "https://vocabreview.uk",
	}, sender)

	response, err := app.RequestMagicLink(context.Background(), "unknown@example.com", "http://evil.test", "")
	if err != nil {
		t.Fatalf("request link: %v", err)
	}
	if response.Message == "" || response.Token != "" || response.VerificationURL != "" || response.ExpiresAt != "" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if len(repo.magicLinks) != 0 {
		t.Fatalf("expected no magic links, got %d", len(repo.magicLinks))
	}
	if len(sender.sends) != 0 {
		t.Fatalf("expected no email sends, got %d", len(sender.sends))
	}
}

func TestProductionMagicLinkForExistingUserSendsEmailWithoutTokenResponse(t *testing.T) {
	repo := newFakeRepository()
	user := domain.User{ID: "usr_existing", Email: "test@example.com", CreatedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)}
	repo.users[user.ID] = user
	repo.usersByEmail[user.Email] = user.ID
	sender := &fakeMagicLinkSender{}
	app := NewAppWithConfig(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)}, nil, AuthConfig{
		Environment:      "production",
		TokenHashSecret:  "secret",
		PublicWebBaseURL: "https://vocabreview.uk/app",
	}, sender)

	response, err := app.RequestMagicLink(context.Background(), "test@example.com", "http://evil.test", "")
	if err != nil {
		t.Fatalf("request link: %v", err)
	}
	if response.Token != "" || response.VerificationURL != "" || response.ExpiresAt != "" {
		t.Fatalf("unexpected token response: %+v", response)
	}
	if len(sender.sends) != 1 {
		t.Fatalf("email sends: got %d want 1", len(sender.sends))
	}
	if !strings.HasPrefix(sender.sends[0].verificationURL, "https://vocabreview.uk/app?token=ml_") {
		t.Fatalf("unexpected verification URL: %q", sender.sends[0].verificationURL)
	}
}

func TestProductionDebugEmailIncludesTokenForExistingUser(t *testing.T) {
	repo := newFakeRepository()
	user := domain.User{ID: "usr_existing", Email: "tester@example.com", CreatedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)}
	repo.users[user.ID] = user
	repo.usersByEmail[user.Email] = user.ID
	app := NewAppWithConfig(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)}, nil, AuthConfig{
		Environment:      "production",
		TokenHashSecret:  "secret",
		PublicWebBaseURL: "https://vocabreview.uk",
		DebugEmails:      []string{"tester@example.com"},
	}, nil)

	response, err := app.RequestMagicLink(context.Background(), "tester@example.com", "http://evil.test", "")
	if err != nil {
		t.Fatalf("request link: %v", err)
	}
	if response.Token == "" || response.VerificationURL == "" || response.ExpiresAt == "" {
		t.Fatalf("expected debug token response, got %+v", response)
	}
	if strings.Contains(response.VerificationURL, "evil.test") {
		t.Fatalf("production used request base URL: %q", response.VerificationURL)
	}
}

func TestProductionEmailIncludesRawTokenForManualClients(t *testing.T) {
	repo := newFakeRepository()
	user := domain.User{ID: "usr_existing", Email: "tester@example.com", CreatedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)}
	repo.users[user.ID] = user
	repo.usersByEmail[user.Email] = user.ID
	sender := &fakeMagicLinkSender{}
	app := NewAppWithConfig(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)}, nil, AuthConfig{
		Environment:      "production",
		TokenHashSecret:  "secret",
		PublicWebBaseURL: "https://vocabreview.uk",
	}, sender)

	_, err := app.RequestMagicLink(context.Background(), "tester@example.com", "http://evil.test", "chrome_extension")
	if err != nil {
		t.Fatalf("request link: %v", err)
	}
	if len(sender.sends) != 1 {
		t.Fatalf("email sends: got %d want 1", len(sender.sends))
	}
	if !strings.HasPrefix(sender.sends[0].verificationURL, "https://vocabreview.uk?token=ml_") {
		t.Fatalf("unexpected verification URL: %q", sender.sends[0].verificationURL)
	}
	if !strings.HasPrefix(sender.sends[0].token, "ml_") {
		t.Fatalf("expected raw token sent to email sender, got %q", sender.sends[0].token)
	}
}

func TestRequestMagicLinkReplacesPendingTokenForEmail(t *testing.T) {
	repo := newFakeRepository()
	app := NewApp(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)})

	first, err := app.RequestMagicLink(context.Background(), "test@example.com", "http://localhost:8080", "")
	if err != nil {
		t.Fatalf("first request link: %v", err)
	}
	second, err := app.RequestMagicLink(context.Background(), "test@example.com", "http://localhost:8080", "")
	if err != nil {
		t.Fatalf("second request link: %v", err)
	}
	if first.Token == second.Token {
		t.Fatal("expected new token on second request")
	}
	if len(repo.magicLinks) != 1 {
		t.Fatalf("magic link count: got %d want 1", len(repo.magicLinks))
	}
	if _, err := app.VerifyMagicLink(context.Background(), first.Token); err == nil || err.Error() != "invalid token" {
		t.Fatalf("old token verify error: got %v want invalid token", err)
	}
	if _, err := app.VerifyMagicLink(context.Background(), second.Token); err != nil {
		t.Fatalf("new token verify: %v", err)
	}
}

type testContextKey struct{}

type fakeEnricher struct {
	ctx         context.Context
	items       []enrichment.Item
	suggestions []enrichment.Suggestion
	err         error
}

func (f *fakeEnricher) Autocomplete(ctx context.Context, items []enrichment.Item) ([]enrichment.Suggestion, error) {
	f.ctx = ctx
	f.items = append([]enrichment.Item(nil), items...)
	return f.suggestions, f.err
}

func TestAutocompleteVocabRequiresConfiguredEnricher(t *testing.T) {
	_, err := newTestApp().AutocompleteVocab(context.Background(), []enrichment.Item{{Term: "serendipity"}})
	if !errors.Is(err, ErrEnrichmentNotConfigured) {
		t.Fatalf("autocomplete error: got %v want %v", err, ErrEnrichmentNotConfigured)
	}
}

func TestAutocompleteVocabUsesConfiguredEnricher(t *testing.T) {
	enricher := &fakeEnricher{
		suggestions: []enrichment.Suggestion{{
			Term:            "serendipity",
			Meaning:         "a fortunate discovery",
			ExampleSentence: "Finding the cafe was pure serendipity.",
			PartOfSpeech:    domain.PartOfSpeechNoun,
		}},
	}
	app := NewAppWithEnricher(newFakeRepository(), stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)}, enricher)
	items := []enrichment.Item{{Term: "serendipity"}}

	suggestions, err := app.AutocompleteVocab(context.Background(), items)
	if err != nil {
		t.Fatalf("autocomplete vocab: %v", err)
	}
	if len(suggestions) != 1 || suggestions[0].Meaning != "a fortunate discovery" {
		t.Fatalf("unexpected suggestions: %#v", suggestions)
	}
	if len(enricher.items) != 1 || enricher.items[0].Term != "serendipity" {
		t.Fatalf("enricher input: %#v", enricher.items)
	}
}

func TestAutocompleteVocabPassesCallerContext(t *testing.T) {
	enricher := &fakeEnricher{}
	app := NewAppWithEnricher(newFakeRepository(), stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)}, enricher)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := app.AutocompleteVocab(ctx, []enrichment.Item{{Term: "serendipity"}})
	if err != nil {
		t.Fatalf("autocomplete vocab: %v", err)
	}
	select {
	case <-enricher.ctx.Done():
	default:
		t.Fatal("expected enricher to receive canceled caller context")
	}
}

func TestCreateVocabPassesCallerContextToRepository(t *testing.T) {
	repo := newFakeRepository()
	app := NewApp(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)})
	ctx := context.WithValue(context.Background(), testContextKey{}, "request-context")

	if _, err := app.CreateVocab(ctx, "usr_test", CreateVocabInput{Term: "serendipity"}); err != nil {
		t.Fatalf("create vocab: %v", err)
	}
	if repo.seenContextValue != "request-context" {
		t.Fatalf("repository context value: got %#v want request-context", repo.seenContextValue)
	}
}

func TestCreateVocabSkipsDuplicateTerms(t *testing.T) {
	repo := newFakeRepository()
	app := NewApp(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)})
	userID := "usr_test"

	first, err := app.CreateVocab(context.Background(), userID, CreateVocabInput{
		Term:    " Serendipity ",
		Meaning: "a happy accident",
		Chinese: "意外發現的美好事物",
	})
	if err != nil {
		t.Fatalf("create first vocab: %v", err)
	}
	if !first.Created || first.SkippedDuplicate {
		t.Fatalf("first result flags: %+v", first)
	}

	duplicate, err := app.CreateVocab(context.Background(), userID, CreateVocabInput{
		Term:    "serendipity",
		Meaning: "different",
	})
	if err != nil {
		t.Fatalf("create duplicate vocab: %v", err)
	}
	if duplicate.Created || !duplicate.SkippedDuplicate {
		t.Fatalf("duplicate result flags: %+v", duplicate)
	}
	if duplicate.Item.ID != first.Item.ID {
		t.Fatalf("duplicate item ID: got %q want %q", duplicate.Item.ID, first.Item.ID)
	}
	if duplicate.Item.Chinese != "意外發現的美好事物" {
		t.Fatalf("duplicate chinese: got %q", duplicate.Item.Chinese)
	}

	listed, err := app.ListVocab(context.Background(), userID, ListVocabInput{})
	if err != nil {
		t.Fatalf("list vocab: %v", err)
	}
	if listed.Total != 1 {
		t.Fatalf("listed total: got %d want 1", listed.Total)
	}
}

func TestReviewScheduling(t *testing.T) {
	app := newTestApp()
	link, err := app.RequestMagicLink(context.Background(), "test@example.com", "http://localhost:8080", "")
	if err != nil {
		t.Fatalf("request link: %v", err)
	}
	auth, err := app.VerifyMagicLink(context.Background(), link.Token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	result, err := app.CreateVocab(context.Background(), auth.User.ID, CreateVocabInput{
		Term:    "serendipity",
		Meaning: "a happy accident",
	})
	if err != nil {
		t.Fatalf("create vocab: %v", err)
	}

	state, err := app.GradeReview(context.Background(), auth.User.ID, result.Item.ID, domain.ReviewGradeGood)
	if err != nil {
		t.Fatalf("good review: %v", err)
	}
	if state.IntervalDays != 1 {
		t.Fatalf("expected interval 1, got %d", state.IntervalDays)
	}

	state, err = app.GradeReview(context.Background(), auth.User.ID, result.Item.ID, domain.ReviewGradeEasy)
	if err != nil {
		t.Fatalf("easy review: %v", err)
	}
	if state.IntervalDays < 3 {
		t.Fatalf("expected easy interval >= 3, got %d", state.IntervalDays)
	}
}

func TestReviewHistoryReturnsRecentReviewedCards(t *testing.T) {
	repo := newFakeRepository()
	app := NewApp(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)})
	link, err := app.RequestMagicLink(context.Background(), "test@example.com", "http://localhost:8080", "")
	if err != nil {
		t.Fatalf("request link: %v", err)
	}
	auth, err := app.VerifyMagicLink(context.Background(), link.Token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	result, err := app.CreateVocab(context.Background(), auth.User.ID, CreateVocabInput{
		Term:    "serendipity",
		Meaning: "a happy accident",
	})
	if err != nil {
		t.Fatalf("create vocab: %v", err)
	}
	if _, err := app.GradeReview(context.Background(), auth.User.ID, result.Item.ID, domain.ReviewGradeGood); err != nil {
		t.Fatalf("grade review: %v", err)
	}

	history, err := app.ReviewHistory(context.Background(), auth.User.ID, PageInput{})
	if err != nil {
		t.Fatalf("review history: %v", err)
	}
	if len(history.Items) != 1 || history.Total != 1 {
		t.Fatalf("expected one history entry, got %+v", history)
	}
	if history.Items[0].Item.ID != result.Item.ID || history.Items[0].Log.Grade != domain.ReviewGradeGood {
		t.Fatalf("unexpected history entry: %+v", history.Items[0])
	}
}

func TestReviewStatsCountsProgressAndCards(t *testing.T) {
	repo := newFakeRepository()
	app := NewApp(repo, stubClock{now: time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)})
	link, err := app.RequestMagicLink(context.Background(), "test@example.com", "http://localhost:8080", "")
	if err != nil {
		t.Fatalf("request link: %v", err)
	}
	auth, err := app.VerifyMagicLink(context.Background(), link.Token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	result, err := app.CreateVocab(context.Background(), auth.User.ID, CreateVocabInput{
		Term:    "serendipity",
		Meaning: "a happy accident",
	})
	if err != nil {
		t.Fatalf("create vocab: %v", err)
	}
	if _, err := app.GradeReview(context.Background(), auth.User.ID, result.Item.ID, domain.ReviewGradeGood); err != nil {
		t.Fatalf("grade review: %v", err)
	}
	archivedResult, err := app.CreateVocab(context.Background(), auth.User.ID, CreateVocabInput{
		Term:    "ephemeral",
		Meaning: "lasting briefly",
	})
	if err != nil {
		t.Fatalf("create archived vocab: %v", err)
	}
	if _, err := app.ArchiveVocab(context.Background(), auth.User.ID, archivedResult.Item.ID); err != nil {
		t.Fatalf("archive vocab: %v", err)
	}

	stats, err := app.ReviewStats(context.Background(), auth.User.ID)
	if err != nil {
		t.Fatalf("review stats: %v", err)
	}
	if stats.ReviewedToday != 1 || stats.Reviewed7Days != 1 || stats.ActiveCards != 1 || stats.DueNow != 0 || stats.ArchivedCards != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestArchiveVocabRemovesCardFromDueReview(t *testing.T) {
	repo := newFakeRepository()
	app := NewApp(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)})
	link, err := app.RequestMagicLink(context.Background(), "test@example.com", "http://localhost:8080", "")
	if err != nil {
		t.Fatalf("request link: %v", err)
	}
	auth, err := app.VerifyMagicLink(context.Background(), link.Token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	result, err := app.CreateVocab(context.Background(), auth.User.ID, CreateVocabInput{
		Term:    "ephemeral",
		Meaning: "lasting briefly",
	})
	if err != nil {
		t.Fatalf("create vocab: %v", err)
	}

	due, err := app.DueCards(context.Background(), auth.User.ID)
	if err != nil {
		t.Fatalf("due cards: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("expected one due card before archive, got %d", len(due))
	}

	archived, err := app.ArchiveVocab(context.Background(), auth.User.ID, result.Item.ID)
	if err != nil {
		t.Fatalf("archive vocab: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("expected archived timestamp")
	}

	listed, err := app.ListVocab(context.Background(), auth.User.ID, ListVocabInput{})
	if err != nil {
		t.Fatalf("list vocab after archive: %v", err)
	}
	if len(listed.Items) != 0 {
		t.Fatalf("expected archived card removed from library, got %d", len(listed.Items))
	}

	due, err = app.DueCards(context.Background(), auth.User.ID)
	if err != nil {
		t.Fatalf("due cards after archive: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("expected archived card removed from due review, got %d", len(due))
	}
}

func TestUpdateVocabClearsPartOfSpeech(t *testing.T) {
	app := newTestApp()
	link, err := app.RequestMagicLink(context.Background(), "test@example.com", "http://localhost:8080", "")
	if err != nil {
		t.Fatalf("request link: %v", err)
	}
	auth, err := app.VerifyMagicLink(context.Background(), link.Token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	noun := domain.PartOfSpeechNoun
	result, err := app.CreateVocab(context.Background(), auth.User.ID, CreateVocabInput{
		Term:         "serendipity",
		Meaning:      "a happy accident",
		PartOfSpeech: &noun,
	})
	if err != nil {
		t.Fatalf("create vocab: %v", err)
	}
	if result.Item.PartOfSpeech != domain.PartOfSpeechNoun {
		t.Fatalf("part of speech after create: got %q want %q", result.Item.PartOfSpeech, domain.PartOfSpeechNoun)
	}

	unspecified := domain.PartOfSpeechUnspecified
	updated, err := app.UpdateVocab(context.Background(), auth.User.ID, result.Item.ID, CreateVocabInput{
		PartOfSpeech: &unspecified,
	})
	if err != nil {
		t.Fatalf("update vocab: %v", err)
	}
	if updated.PartOfSpeech != domain.PartOfSpeechUnspecified {
		t.Fatalf("part of speech after clear: got %q want %q", updated.PartOfSpeech, domain.PartOfSpeechUnspecified)
	}
}
