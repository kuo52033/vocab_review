package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
	"vocabreview/backend/internal/service/audios"
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
	audios           map[string]domain.VocabAudio
	audioJobs        map[string]domain.VocabAudioJob
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
		audios:           map[string]domain.VocabAudio{},
		audioJobs:        map[string]domain.VocabAudioJob{},
		deviceTokens:     map[string]domain.DeviceToken{},
		notificationJobs: map[string]domain.NotificationJob{},
	}
}

func (f *fakeRepository) HealthCheck(context.Context) error { return nil }

func (f *fakeRepository) PutMagicLink(_ context.Context, token domain.MagicLinkToken, minInterval time.Duration) (bool, error) {
	for _, existing := range f.magicLinks {
		if token.CreatedAt.After(existing.ExpiresAt) {
			continue
		}
		if existing.Email == token.Email && minInterval > 0 && token.CreatedAt.Sub(existing.CreatedAt) < minInterval {
			return false, nil
		}
	}
	for tokenHash, existing := range f.magicLinks {
		if token.CreatedAt.After(existing.ExpiresAt) {
			delete(f.magicLinks, tokenHash)
			continue
		}
		if existing.Email == token.Email {
			delete(f.magicLinks, tokenHash)
		}
	}
	f.magicLinks[token.TokenHash] = token
	return true, nil
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

func (f *fakeRepository) CreateVocab(ctx context.Context, item domain.VocabItem, state domain.ReviewState, job *domain.NotificationJob, audioJob *domain.VocabAudioJob) error {
	f.seenContextValue = ctx.Value(testContextKey{})
	f.vocab[item.ID] = item
	f.reviewStates[state.VocabItemID] = state
	if job != nil {
		f.notificationJobs[job.ID] = *job
	}
	if audioJob != nil {
		f.audioJobs[audioJob.VocabItemID] = *audioJob
	}
	return nil
}

func (f *fakeRepository) CreateVocabBatch(ctx context.Context, creates []repository.VocabCreate) error {
	for _, create := range creates {
		if err := f.CreateVocab(ctx, create.Item, create.State, create.Job, create.AudioJob); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeRepository) CreateCapturedVocab(ctx context.Context, item domain.VocabItem, state domain.ReviewState, capture domain.CaptureSource, job *domain.NotificationJob, audioJob *domain.VocabAudioJob) error {
	if err := f.CreateVocab(ctx, item, state, job, audioJob); err != nil {
		return err
	}
	f.captures[capture.ID] = capture
	return nil
}

func (f *fakeRepository) GetVocab(_ context.Context, id string) (domain.VocabItem, bool, error) {
	item, ok := f.vocab[id]
	if item.AudioID != "" {
		audio, audioOK := f.audios[item.AudioID]
		if audioOK {
			item.Audio = &audio
		}
	}
	return item, ok, nil
}

func (f *fakeRepository) UpdateVocab(_ context.Context, item domain.VocabItem, audioJob *domain.VocabAudioJob) error {
	f.vocab[item.ID] = item
	if audioJob != nil {
		f.audioJobs[audioJob.VocabItemID] = *audioJob
	}
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

func (f *fakeRepository) ListActiveVocabByTerms(_ context.Context, userID string, terms []string) ([]repository.VocabWithState, error) {
	termSet := make(map[string]struct{}, len(terms))
	for _, term := range terms {
		termSet[strings.ToLower(strings.TrimSpace(term))] = struct{}{}
	}
	items := make([]repository.VocabWithState, 0)
	for _, item := range f.vocab {
		if item.UserID != userID || item.ArchivedAt != nil {
			continue
		}
		if _, ok := termSet[strings.ToLower(strings.TrimSpace(item.Term))]; !ok {
			continue
		}
		items = append(items, repository.VocabWithState{Item: item, State: f.reviewStates[item.ID]})
	}
	return items, nil
}

func (f *fakeRepository) ListVocabByUser(_ context.Context, userID string, options repository.ListVocabOptions) ([]repository.VocabWithState, int, bool, error) {
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
			if !strings.Contains(strings.ToLower(item.Term), strings.ToLower(options.Query)) {
				continue
			}
		}
		items = append(items, repository.VocabWithState{Item: item, State: state})
	}
	total := len(items)
	if options.Offset > len(items) {
		return []repository.VocabWithState{}, total, false, nil
	}
	if options.Offset > 0 {
		items = items[options.Offset:]
	}
	hasNext := options.Limit > 0 && len(items) > options.Limit
	if options.Limit > 0 && len(items) > options.Limit {
		items = items[:options.Limit]
	}
	return items, total, hasNext, nil
}

func (f *fakeRepository) ListDueVocab(_ context.Context, userID string, now time.Time, limit int) ([]repository.VocabWithState, error) {
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
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (f *fakeRepository) ListReviewSessionCandidates(_ context.Context, userID string, limit int) ([]repository.ReviewSessionCandidate, error) {
	candidates := make([]repository.ReviewSessionCandidate, 0)
	for _, item := range f.vocab {
		if item.UserID != userID || item.ArchivedAt != nil || strings.TrimSpace(item.Meaning) == "" {
			continue
		}
		candidates = append(candidates, repository.ReviewSessionCandidate{
			ID:      item.ID,
			Term:    item.Term,
			Meaning: item.Meaning,
			Chinese: item.Chinese,
		})
	}
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func (f *fakeRepository) GetReviewSessionData(ctx context.Context, userID string, now time.Time, dueLimit int, candidateLimit int) (repository.ReviewSessionData, error) {
	due, err := f.ListDueVocab(ctx, userID, now, dueLimit)
	if err != nil {
		return repository.ReviewSessionData{}, err
	}
	candidates, err := f.ListReviewSessionCandidates(ctx, userID, candidateLimit)
	if err != nil {
		return repository.ReviewSessionData{}, err
	}
	stats, err := f.GetReviewStats(ctx, userID, now)
	if err != nil {
		return repository.ReviewSessionData{}, err
	}
	return repository.ReviewSessionData{Due: due, Candidates: candidates, Stats: stats}, nil
}

func (f *fakeRepository) GetReviewState(_ context.Context, vocabID string) (domain.ReviewState, bool, error) {
	state, ok := f.reviewStates[vocabID]
	return state, ok, nil
}

func (f *fakeRepository) GetReadyVocabAudio(_ context.Context, provider, model, voice string, speed float64, outputFormat, inputHash string) (domain.VocabAudio, bool, error) {
	for _, audio := range f.audios {
		if audio.Provider == provider && audio.Model == model && audio.Voice == voice && audio.Speed == speed && audio.OutputFormat == outputFormat && audio.InputHash == inputHash && audio.Status == "ready" {
			return audio, true, nil
		}
	}
	return domain.VocabAudio{}, false, nil
}

func (f *fakeRepository) ClaimPendingVocabAudioJobs(_ context.Context, _ time.Time, _ int) ([]domain.VocabAudioJob, error) {
	return nil, nil
}

func (f *fakeRepository) CompleteVocabAudioJob(_ context.Context, job domain.VocabAudioJob, audio domain.VocabAudio) (domain.VocabAudio, bool, error) {
	f.audios[audio.ID] = audio
	item := f.vocab[job.VocabItemID]
	item.AudioID = audio.ID
	f.vocab[item.ID] = item
	job.Status = "ready"
	job.AudioID = audio.ID
	f.audioJobs[job.VocabItemID] = job
	return audio, true, nil
}

func (f *fakeRepository) MarkVocabAudioJobFailed(_ context.Context, jobID string, nextAttemptAt time.Time, lastError string) error {
	for vocabID, job := range f.audioJobs {
		if job.ID != jobID {
			continue
		}
		job.Status = "failed"
		if job.AttemptCount < job.MaxAttempts {
			job.Status = "pending"
		}
		job.NextAttemptAt = nextAttemptAt
		job.LastError = lastError
		f.audioJobs[vocabID] = job
	}
	return nil
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

func (f *fakeRepository) ListReviewHistory(_ context.Context, userID string, pagination repository.Pagination) ([]repository.ReviewHistoryEntry, int, bool, error) {
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
		return []repository.ReviewHistoryEntry{}, total, false, nil
	}
	if pagination.Offset > 0 {
		entries = entries[pagination.Offset:]
	}
	hasNext := pagination.Limit > 0 && len(entries) > pagination.Limit
	if pagination.Limit > 0 && len(entries) > pagination.Limit {
		entries = entries[:pagination.Limit]
	}
	return entries, total, hasNext, nil
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

func testAudioConfig() VocabAudioConfig {
	return VocabAudioConfig{
		Enabled:       true,
		Provider:      "openai",
		Model:         "gpt-4o-mini-tts",
		Voice:         "alloy",
		Speed:         1,
		OutputFormat:  "mp3",
		PublicBaseURL: "https://cdn.example.com",
	}
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

func TestProductionDebugEmailIncludesTokenForExistingUserWithoutSendingEmail(t *testing.T) {
	repo := newFakeRepository()
	user := domain.User{ID: "usr_existing", Email: "tester@example.com", CreatedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)}
	repo.users[user.ID] = user
	repo.usersByEmail[user.Email] = user.ID
	sender := &fakeMagicLinkSender{}
	app := NewAppWithConfig(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)}, nil, AuthConfig{
		Environment:      "production",
		TokenHashSecret:  "secret",
		PublicWebBaseURL: "https://vocabreview.uk",
		DebugEmails:      []string{"tester@example.com"},
	}, sender)

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
	if len(sender.sends) != 0 {
		t.Fatalf("expected debug email to skip sends, got %d", len(sender.sends))
	}
}

func TestProductionMagicLinkRateLimitSuppressesEmail(t *testing.T) {
	repo := newFakeRepository()
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	user := domain.User{ID: "usr_existing", Email: "test@example.com", CreatedAt: now.Add(-time.Hour)}
	repo.users[user.ID] = user
	repo.usersByEmail[user.Email] = user.ID
	sender := &fakeMagicLinkSender{}
	app := NewAppWithConfig(repo, stubClock{now: now}, nil, AuthConfig{
		Environment:      "production",
		TokenHashSecret:  "secret",
		PublicWebBaseURL: "https://vocabreview.uk",
	}, sender)

	first, err := app.RequestMagicLink(context.Background(), "test@example.com", "http://evil.test", "")
	if err != nil {
		t.Fatalf("first request link: %v", err)
	}
	second, err := app.RequestMagicLink(context.Background(), "test@example.com", "http://evil.test", "")
	if err != nil {
		t.Fatalf("second request link: %v", err)
	}
	if first.Token != "" || first.VerificationURL != "" || first.ExpiresAt != "" {
		t.Fatalf("expected first production response to hide token: %+v", first)
	}
	if second.Token != "" || second.VerificationURL != "" || second.ExpiresAt != "" {
		t.Fatalf("expected generic rate-limited response, got %+v", second)
	}
	if len(sender.sends) != 1 {
		t.Fatalf("email sends: got %d want 1", len(sender.sends))
	}
	if len(repo.magicLinks) != 1 {
		t.Fatalf("magic links: got %d want 1", len(repo.magicLinks))
	}
}

func TestProductionDebugEmailCanRequestTokenRepeatedlyWithoutSendingEmail(t *testing.T) {
	repo := newFakeRepository()
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	user := domain.User{ID: "usr_existing", Email: "tester@example.com", CreatedAt: now.Add(-time.Hour)}
	repo.users[user.ID] = user
	repo.usersByEmail[user.Email] = user.ID
	sender := &fakeMagicLinkSender{}
	app := NewAppWithConfig(repo, stubClock{now: now}, nil, AuthConfig{
		Environment:      "production",
		TokenHashSecret:  "secret",
		PublicWebBaseURL: "https://vocabreview.uk",
		DebugEmails:      []string{"tester@example.com"},
	}, sender)

	first, err := app.RequestMagicLink(context.Background(), "tester@example.com", "http://evil.test", "")
	if err != nil {
		t.Fatalf("first request link: %v", err)
	}
	second, err := app.RequestMagicLink(context.Background(), "tester@example.com", "http://evil.test", "")
	if err != nil {
		t.Fatalf("second request link: %v", err)
	}
	if first.Token == "" {
		t.Fatalf("expected first debug response to include token: %+v", first)
	}
	if second.Token == "" || second.VerificationURL == "" || second.ExpiresAt == "" {
		t.Fatalf("expected second debug response to include token: %+v", second)
	}
	if second.Token == first.Token {
		t.Fatal("expected second debug request to issue a fresh token")
	}
	if len(sender.sends) != 0 {
		t.Fatalf("email sends: got %d want 0", len(sender.sends))
	}
	if len(repo.magicLinks) != 1 {
		t.Fatalf("magic links: got %d want 1", len(repo.magicLinks))
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

type fakeAudioURLSigner struct {
	storageKey string
	url        string
}

func (s *fakeAudioURLSigner) SignVocabAudioURL(_ context.Context, storageKey string) (string, error) {
	s.storageKey = storageKey
	return s.url, nil
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

func TestCreateVocabReturnsUnavailableAudioWhenTTSDisabled(t *testing.T) {
	app := NewApp(newFakeRepository(), stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)})

	result, err := app.CreateVocab(context.Background(), "usr_test", CreateVocabInput{Term: "Serendipity"})
	if err != nil {
		t.Fatalf("create vocab: %v", err)
	}
	if result.Item.Audio == nil || result.Item.Audio.Status != "unavailable" {
		t.Fatalf("audio status: %+v", result.Item.Audio)
	}
}

func TestCreateVocabEnqueuesAudioWhenConfigured(t *testing.T) {
	repo := newFakeRepository()
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	app := NewAppWithVocabAudioConfig(repo, stubClock{now: now}, nil, AuthConfig{Environment: "development"}, nil, testAudioConfig())

	result, err := app.CreateVocab(context.Background(), "usr_test", CreateVocabInput{Term: " Serendipity "})
	if err != nil {
		t.Fatalf("create vocab: %v", err)
	}
	job, ok := repo.audioJobs[result.Item.ID]
	if !ok {
		t.Fatal("expected audio job")
	}
	if job.InputText != "serendipity" || job.Status != "pending" || job.Speed != 1 {
		t.Fatalf("unexpected audio job: %+v", job)
	}
	if result.Item.Audio == nil || result.Item.Audio.Status != "pending" {
		t.Fatalf("audio response: %+v", result.Item.Audio)
	}
}

func TestBulkCreateVocabCreatesAndSkipsDuplicates(t *testing.T) {
	repo := newFakeRepository()
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	app := NewAppWithVocabAudioConfig(repo, stubClock{now: now}, nil, AuthConfig{Environment: "development"}, nil, testAudioConfig())

	existing, err := app.CreateVocab(context.Background(), "usr_test", CreateVocabInput{Term: "existing"})
	if err != nil {
		t.Fatalf("create existing: %v", err)
	}

	result, err := app.BulkCreateVocab(context.Background(), "usr_test", BulkCreateVocabInput{Items: []CreateVocabInput{
		{Term: "alpha", Meaning: "first"},
		{Term: "existing"},
		{Term: "Alpha", Meaning: "duplicate in request"},
		{Term: "beta", Meaning: "second"},
	}})
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if result.CreatedCount != 2 || result.SkippedDuplicateCount != 2 || len(result.Items) != 4 {
		t.Fatalf("bulk result: %+v", result)
	}
	if !result.AudioJobEnqueued {
		t.Fatal("expected audio job enqueued")
	}
	if !result.Items[0].Created || !result.Items[3].Created {
		t.Fatalf("expected alpha and beta created: %+v", result.Items)
	}
	if !result.Items[1].SkippedDuplicate || result.Items[1].Item.ID != existing.Item.ID {
		t.Fatalf("expected existing duplicate: %+v", result.Items[1])
	}
	if !result.Items[2].SkippedDuplicate || result.Items[2].Item.Term != "alpha" {
		t.Fatalf("expected duplicate to reference first created item: %+v", result.Items[2])
	}
	if len(repo.audioJobs) != 3 {
		t.Fatalf("audio jobs: got %d want 3", len(repo.audioJobs))
	}
}

func TestCreateVocabReusesReadyAudio(t *testing.T) {
	repo := newFakeRepository()
	config := testAudioConfig()
	inputHash := audios.InputHash(audioGenerationConfig(config), "serendipity")
	repo.audios["aud_existing"] = domain.VocabAudio{
		ID:           "aud_existing",
		Provider:     config.Provider,
		Model:        config.Model,
		Voice:        config.Voice,
		Speed:        config.Speed,
		OutputFormat: config.OutputFormat,
		InputHash:    inputHash,
		StorageKey:   "audio/openai/gpt-4o-mini-tts/alloy/" + inputHash + ".mp3",
		Status:       "ready",
	}
	app := NewAppWithVocabAudioConfig(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)}, nil, AuthConfig{Environment: "development"}, nil, config)

	result, err := app.CreateVocab(context.Background(), "usr_test", CreateVocabInput{Term: "Serendipity"})
	if err != nil {
		t.Fatalf("create vocab: %v", err)
	}
	if result.Item.AudioID != "aud_existing" || result.Item.Audio == nil || result.Item.Audio.Status != "ready" {
		t.Fatalf("audio response: item=%+v audio=%+v", result.Item, result.Item.Audio)
	}
	if len(repo.audioJobs) != 0 {
		t.Fatalf("expected no audio jobs, got %+v", repo.audioJobs)
	}
}

func TestUpdateVocabEnqueuesNewAudioWhenTermChanges(t *testing.T) {
	repo := newFakeRepository()
	config := testAudioConfig()
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	app := NewAppWithVocabAudioConfig(repo, stubClock{now: now}, nil, AuthConfig{Environment: "development"}, nil, config)
	created, err := app.CreateVocab(context.Background(), "usr_test", CreateVocabInput{Term: "first"})
	if err != nil {
		t.Fatalf("create vocab: %v", err)
	}

	updated, err := app.UpdateVocab(context.Background(), "usr_test", created.Item.ID, CreateVocabInput{Term: "second"})
	if err != nil {
		t.Fatalf("update vocab: %v", err)
	}
	job := repo.audioJobs[created.Item.ID]
	if job.InputText != "second" {
		t.Fatalf("audio job input: got %q want second", job.InputText)
	}
	if updated.Item.Audio == nil || updated.Item.Audio.Status != "pending" {
		t.Fatalf("updated audio: %+v", updated.Item.Audio)
	}
	if !updated.AudioJobEnqueued {
		t.Fatal("expected update to report audio job enqueued")
	}
}

func TestVocabAudioURLSignsReadyPrivateAudio(t *testing.T) {
	repo := newFakeRepository()
	signer := &fakeAudioURLSigner{url: "https://signed.example.com/audio.mp3"}
	config := testAudioConfig()
	config.PublicBaseURL = ""
	app := NewAppWithVocabAudioConfigAndSigner(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)}, nil, AuthConfig{Environment: "development"}, nil, config, signer)
	userID := "usr_test"
	result, err := app.CreateVocab(context.Background(), userID, CreateVocabInput{Term: "serendipity"})
	if err != nil {
		t.Fatalf("create vocab: %v", err)
	}
	audio := domain.VocabAudio{
		ID:           "aud_ready",
		Provider:     config.Provider,
		Model:        config.Model,
		Voice:        config.Voice,
		Speed:        config.Speed,
		OutputFormat: config.OutputFormat,
		InputHash:    "hash",
		StorageKey:   "audio/openai/gpt-4o-mini-tts/alloy/hash.mp3",
		Status:       "ready",
	}
	repo.audios[audio.ID] = audio
	item := repo.vocab[result.Item.ID]
	item.AudioID = audio.ID
	repo.vocab[item.ID] = item

	url, err := app.VocabAudioURL(context.Background(), userID, result.Item.ID)
	if err != nil {
		t.Fatalf("vocab audio url: %v", err)
	}
	if url != signer.url {
		t.Fatalf("url: got %q want %q", url, signer.url)
	}
	if signer.storageKey != audio.StorageKey {
		t.Fatalf("signed key: got %q want %q", signer.storageKey, audio.StorageKey)
	}
}

func TestVocabAudioURLRequiresReadyAudio(t *testing.T) {
	repo := newFakeRepository()
	app := NewAppWithVocabAudioConfig(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)}, nil, AuthConfig{Environment: "development"}, nil, testAudioConfig())
	result, err := app.CreateVocab(context.Background(), "usr_test", CreateVocabInput{Term: "serendipity"})
	if err != nil {
		t.Fatalf("create vocab: %v", err)
	}

	_, err = app.VocabAudioURL(context.Background(), "usr_test", result.Item.ID)
	if !errors.Is(err, ErrVocabAudioNotReady) {
		t.Fatalf("error: got %v want %v", err, ErrVocabAudioNotReady)
	}
}

func TestCreateCaptureSkipsDuplicateTerms(t *testing.T) {
	repo := newFakeRepository()
	app := NewApp(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)})
	userID := "usr_test"

	first, err := app.CreateCapture(context.Background(), userID, CaptureInput{
		Term:    " Serendipity ",
		Meaning: "a happy accident",
		Chinese: "意外發現的美好事物",
		PageURL: "https://example.com/first",
	})
	if err != nil {
		t.Fatalf("create first capture: %v", err)
	}

	duplicate, err := app.CreateCapture(context.Background(), userID, CaptureInput{
		Term:    "serendipity",
		Meaning: "different",
		PageURL: "https://example.com/second",
	})
	if err != nil {
		t.Fatalf("create duplicate capture: %v", err)
	}
	if duplicate.Item.ID != first.Item.ID {
		t.Fatalf("duplicate item ID: got %q want %q", duplicate.Item.ID, first.Item.ID)
	}
	if duplicate.Item.Chinese != "意外發現的美好事物" {
		t.Fatalf("duplicate chinese: got %q", duplicate.Item.Chinese)
	}
	if len(repo.vocab) != 1 {
		t.Fatalf("vocab count: got %d want 1", len(repo.vocab))
	}
	if len(repo.captures) != 1 {
		t.Fatalf("capture count: got %d want 1", len(repo.captures))
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
	if updated.Item.PartOfSpeech != domain.PartOfSpeechUnspecified {
		t.Fatalf("part of speech after clear: got %q want %q", updated.Item.PartOfSpeech, domain.PartOfSpeechUnspecified)
	}
}
