package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"
	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/service"
)

type routeHTTPRepository struct {
	authenticatedHTTPRepository

	vocab       domain.VocabItem
	reviewState domain.ReviewState

	seenGetVocabID      string
	seenUpdateVocab     domain.VocabItem
	seenArchiveUserID   string
	seenArchiveVocabID  string
	seenReviewVocabID   string
	seenRecordState     domain.ReviewState
	seenRecordReviewLog domain.ReviewLog
}

func (r *routeHTTPRepository) GetVocab(_ context.Context, id string) (domain.VocabItem, bool, error) {
	r.seenGetVocabID = id

	if r.vocab.ID != id {
		return domain.VocabItem{}, false, nil
	}

	return r.vocab, true, nil
}

func (r *routeHTTPRepository) UpdateVocab(_ context.Context, item domain.VocabItem, _ *domain.VocabAudioJob) error {
	r.seenUpdateVocab = item
	return nil
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
