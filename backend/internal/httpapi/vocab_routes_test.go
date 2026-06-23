package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
	"vocabreview/backend/internal/service"
)

type routeHTTPRepository struct {
	authenticatedHTTPRepository

	vocab       domain.VocabItem
	reviewState domain.ReviewState

	seenCreateVocab     domain.VocabItem
	seenCreateAudioJob  *domain.VocabAudioJob
	seenCreateBatch     []repository.VocabCreate
	seenListOptions     repository.ListVocabOptions
	seenListDueUserID   string
	seenStatsUserID     string
	seenGetVocabID      string
	seenUpdateVocab     domain.VocabItem
	seenUpdateAudioJob  *domain.VocabAudioJob
	seenArchiveUserID   string
	seenArchiveVocabID  string
	seenReviewVocabID   string
	seenRecordState     domain.ReviewState
	seenRecordReviewLog domain.ReviewLog
}

func (r *routeHTTPRepository) CreateVocab(_ context.Context, item domain.VocabItem, _ domain.ReviewState, _ *domain.NotificationJob, audioJob *domain.VocabAudioJob) error {
	r.seenCreateVocab = item
	r.seenCreateAudioJob = audioJob
	return nil
}

func (r *routeHTTPRepository) CreateVocabBatch(_ context.Context, creates []repository.VocabCreate) error {
	r.seenCreateBatch = creates
	return nil
}

func (r *routeHTTPRepository) GetActiveVocabByTerm(context.Context, string, string) (repository.VocabWithState, bool, error) {
	return repository.VocabWithState{}, false, nil
}

func (r *routeHTTPRepository) ListActiveVocabByTerms(context.Context, string, []string) ([]repository.VocabWithState, error) {
	return nil, nil
}

func (r *routeHTTPRepository) ListVocabByUser(_ context.Context, userID string, options repository.ListVocabOptions) ([]repository.VocabWithState, int, bool, error) {
	r.seenListOptions = options
	return []repository.VocabWithState{
		{
			Item:  domain.VocabItem{ID: "voc_boot", UserID: userID, Term: "bootstrap"},
			State: domain.ReviewState{VocabItemID: "voc_boot", UserID: userID},
		},
	}, 7, true, nil
}

func (r *routeHTTPRepository) ListDueVocab(_ context.Context, userID string, _ time.Time) ([]repository.VocabWithState, error) {
	r.seenListDueUserID = userID
	return []repository.VocabWithState{
		{
			Item:  domain.VocabItem{ID: "voc_due", UserID: userID, Term: "due"},
			State: domain.ReviewState{VocabItemID: "voc_due", UserID: userID},
		},
	}, nil
}

func (r *routeHTTPRepository) GetReviewStats(_ context.Context, userID string, _ time.Time) (repository.ReviewStats, error) {
	r.seenStatsUserID = userID
	return repository.ReviewStats{ActiveCards: 7, DueNow: 1}, nil
}

func (r *routeHTTPRepository) GetVocab(_ context.Context, id string) (domain.VocabItem, bool, error) {
	r.seenGetVocabID = id

	if r.vocab.ID != id {
		return domain.VocabItem{}, false, nil
	}

	return r.vocab, true, nil
}

func (r *routeHTTPRepository) UpdateVocab(_ context.Context, item domain.VocabItem, audioJob *domain.VocabAudioJob) error {
	r.seenUpdateVocab = item
	r.seenUpdateAudioJob = audioJob
	return nil
}

func (r *routeHTTPRepository) GetReadyVocabAudio(context.Context, string, string, string, float64, string, string) (domain.VocabAudio, bool, error) {
	return domain.VocabAudio{}, false, nil
}

func (r *routeHTTPRepository) ArchiveVocabForUser(ctx context.Context, userID string, vocabID string, archivedAt time.Time) (domain.VocabItem, error) {
	r.seenArchiveUserID = userID
	r.seenArchiveVocabID = vocabID
	r.vocab.ArchivedAt = &archivedAt
	return r.vocab, nil
}

func (r *routeHTTPRepository) GetReviewState(_ context.Context, vocabID string) (domain.ReviewState, bool, error) {
	r.seenReviewVocabID = vocabID
	if r.reviewState.VocabItemID != vocabID {
		return domain.ReviewState{}, false, nil
	}
	return r.reviewState, true, nil
}

func (r *routeHTTPRepository) RecordReview(_ context.Context, state domain.ReviewState, log domain.ReviewLog, _ *domain.NotificationJob) error {
	r.seenRecordState = state
	r.seenRecordReviewLog = log
	return nil
}

type fakeAudioWorkerWake struct {
	calls int
}

func (w *fakeAudioWorkerWake) Wake(context.Context) error {
	w.calls++
	return nil
}

func TestCreateVocabWakesAudioWorkerWhenAudioJobEnqueued(t *testing.T) {
	repo := &routeHTTPRepository{}
	app := service.NewAppWithVocabAudioConfig(repo, clock.RealClock{}, nil, service.AuthConfig{Environment: "development"}, nil, testWakeAudioConfig())
	wake := &fakeAudioWorkerWake{}
	handler := NewServerWithAudioWorkerWake(app, testLogger(), wake).Handler()

	response := performRequest(handler, authenticatedRequest(
		http.MethodPost,
		"/vocab",
		bytes.NewBufferString(`{"term":"serendipity"}`),
	))

	if response.Code != http.StatusCreated {
		t.Fatalf("status: got %d want %d", response.Code, http.StatusCreated)
	}
	if repo.seenCreateAudioJob == nil {
		t.Fatal("expected audio job to be enqueued")
	}
	if wake.calls != 1 {
		t.Fatalf("wake calls: got %d want 1", wake.calls)
	}
}

func TestCreateVocabDoesNotWakeAudioWorkerWithoutAudioJob(t *testing.T) {
	repo := &routeHTTPRepository{}
	app := service.NewApp(repo, clock.RealClock{})
	wake := &fakeAudioWorkerWake{}
	handler := NewServerWithAudioWorkerWake(app, testLogger(), wake).Handler()

	response := performRequest(handler, authenticatedRequest(
		http.MethodPost,
		"/vocab",
		bytes.NewBufferString(`{"term":"serendipity"}`),
	))

	if response.Code != http.StatusCreated {
		t.Fatalf("status: got %d want %d", response.Code, http.StatusCreated)
	}
	if wake.calls != 0 {
		t.Fatalf("wake calls: got %d want 0", wake.calls)
	}
}

func TestBulkCreateVocabWakesAudioWorkerOnce(t *testing.T) {
	repo := &routeHTTPRepository{}
	app := service.NewAppWithVocabAudioConfig(repo, clock.RealClock{}, nil, service.AuthConfig{Environment: "development"}, nil, testWakeAudioConfig())
	wake := &fakeAudioWorkerWake{}
	handler := NewServerWithAudioWorkerWake(app, testLogger(), wake).Handler()

	response := performRequest(handler, authenticatedRequest(
		http.MethodPost,
		"/vocab/bulk",
		bytes.NewBufferString(`{"items":[{"term":"alpha"},{"term":"beta"}]}`),
	))

	if response.Code != http.StatusCreated {
		t.Fatalf("status: got %d want %d body %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if len(repo.seenCreateBatch) != 2 {
		t.Fatalf("batch creates: got %d want 2", len(repo.seenCreateBatch))
	}
	if wake.calls != 1 {
		t.Fatalf("wake calls: got %d want 1", wake.calls)
	}
	var body service.BulkCreateVocabResult
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.CreatedCount != 2 || body.SkippedDuplicateCount != 0 || len(body.Items) != 2 {
		t.Fatalf("bulk body: %+v", body)
	}
}

func testWakeAudioConfig() service.VocabAudioConfig {
	return service.VocabAudioConfig{
		Enabled:      true,
		Provider:     "openai",
		Model:        "gpt-4o-mini-tts",
		Voice:        "alloy",
		Speed:        1,
		OutputFormat: "mp3",
	}
}

func TestAppBootstrapCombinesInitialData(t *testing.T) {
	repo := &routeHTTPRepository{}
	handler := NewServer(service.NewApp(repo, clock.RealClock{}), testLogger()).Handler()
	request := authenticatedRequest(http.MethodGet, "/app/bootstrap?limit=10&offset=20&q=seren&status=learning", nil)
	response := performRequest(handler, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d body %s", response.Code, http.StatusOK, response.Body.String())
	}
	if repo.seenListOptions.Limit != 10 || repo.seenListOptions.Offset != 20 || repo.seenListOptions.Query != "seren" || repo.seenListOptions.Status != domain.ReviewStatusLearning {
		t.Fatalf("list options: %+v", repo.seenListOptions)
	}
	if repo.seenListDueUserID != testUserID || repo.seenStatsUserID != testUserID {
		t.Fatalf("user IDs: due=%q stats=%q", repo.seenListDueUserID, repo.seenStatsUserID)
	}
	var body service.AppBootstrap
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Library.Total != 7 || !body.Library.HasNext || len(body.Library.Items) != 1 || len(body.Due) != 1 || body.Stats.ActiveCards != 7 {
		t.Fatalf("bootstrap body: %+v", body)
	}
}

func TestPatchVocabRouteUsesPathID(t *testing.T) {
	repo := &routeHTTPRepository{
		vocab: domain.VocabItem{
			ID:      "voc_route",
			UserID:  testUserID,
			Term:    "old term",
			Meaning: "old meaning",
		},
	}
	app := service.NewApp(repo, clock.RealClock{})
	handler := NewServer(app, testLogger()).Handler()
	request := authenticatedRequest(
		http.MethodPatch,
		"/vocab/voc_route",
		bytes.NewBufferString(`{"term":"new term","meaning":"new meaning"}`),
	)
	response := performRequest(handler, request)

	if response.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, response.Code)
	}

	if repo.seenGetVocabID != "voc_route" {
		t.Fatalf("GetVocab id: got %q want %q", repo.seenGetVocabID, "voc_route")
	}

	if repo.seenUpdateVocab.ID != "voc_route" {
		t.Fatalf("updated vocab id: got %q want %q", repo.seenUpdateVocab.ID, "voc_route")
	}

	if repo.seenUpdateVocab.UserID != testUserID {
		t.Fatalf("updated vocab userID: got %q want %q", repo.seenUpdateVocab.UserID, testUserID)
	}
}

func TestDeleteVocabRouteUsesPathID(t *testing.T) {
	repo := &routeHTTPRepository{
		vocab: domain.VocabItem{
			ID:      "voc_route",
			UserID:  testUserID,
			Term:    "old term",
			Meaning: "old meaning",
		},
	}
	app := service.NewApp(repo, clock.RealClock{})
	handler := NewServer(app, testLogger()).Handler()
	request := authenticatedRequest(
		http.MethodDelete,
		"/vocab/voc_route",
		nil,
	)
	response := performRequest(handler, request)

	if response.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, response.Code)
	}

	if repo.seenArchiveVocabID != "voc_route" {
		t.Fatalf("ArchiveVocab id: got %q want %q", repo.seenArchiveVocabID, "voc_route")
	}

	if repo.seenArchiveUserID != testUserID {
		t.Fatalf("ArchiveVocab userID: got %q want %q", repo.seenArchiveUserID, testUserID)
	}
}

func TestPostReviewGradeRouteUsesPathID(t *testing.T) {
	repo := &routeHTTPRepository{
		reviewState: domain.ReviewState{
			VocabItemID:     "voc_route",
			UserID:          testUserID,
			Status:          domain.ReviewStatusLearning,
			EaseFactor:      2.5,
			IntervalDays:    0,
			RepetitionCount: 0,
			NextDueAt:       time.Now().UTC(),
		},
	}
	app := service.NewApp(repo, clock.RealClock{})
	handler := NewServer(app, testLogger()).Handler()
	request := authenticatedRequest(
		http.MethodPost,
		"/reviews/voc_route/grade",
		bytes.NewBufferString(`{"grade": "good"}`),
	)
	response := performRequest(handler, request)

	if response.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, response.Code)
	}

	if repo.seenReviewVocabID != "voc_route" {
		t.Fatalf("GradeReview id: got %q want %q", repo.seenReviewVocabID, "voc_route")
	}

	if repo.seenRecordReviewLog.VocabItemID != "voc_route" {
		t.Fatalf("RecordReview log vocab id: got %q want %q", repo.seenRecordReviewLog.VocabItemID, "voc_route")
	}

	if repo.seenRecordReviewLog.Grade != domain.ReviewGradeGood {
		t.Fatalf("RecordReview log grade: got %q want %q", repo.seenRecordReviewLog.Grade, domain.ReviewGradeGood)
	}
}

func TestIDBearingRoutesRejectMalformedPaths(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"patch vocab extra segment", http.MethodPatch, "/vocab/voc_route/extra", `{"term":"x"}`},
		{"delete vocab extra segment", http.MethodDelete, "/vocab/voc_route/extra", ""},
		{"grade review extra segment", http.MethodPost, "/reviews/voc_route/grade/extra", `{"grade":"good"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &routeHTTPRepository{}
			handler := NewServer(service.NewApp(repo, clock.RealClock{}), testLogger()).Handler()

			response := performRequest(
				handler,
				authenticatedRequest(tc.method, tc.path, bytes.NewBufferString(tc.body)),
			)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status: got %d want %d body %s", response.Code, http.StatusNotFound, response.Body.String())
			}
			if repo.seenGetVocabID != "" || repo.seenArchiveVocabID != "" || repo.seenReviewVocabID != "" {
				t.Fatalf("repository was hit for malformed route: %+v", repo)
			}
		})
	}
}
