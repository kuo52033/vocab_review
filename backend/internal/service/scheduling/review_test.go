package scheduling

import (
	"testing"
	"time"

	"vocabreview/backend/internal/domain"
)

func TestScheduleReviewAppliesGradeTransitions(t *testing.T) {
	reviewedAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	previousReview := reviewedAt.Add(-24 * time.Hour)

	tests := []struct {
		name             string
		current          domain.ReviewState
		grade            domain.ReviewGrade
		wantStatus       domain.ReviewStatus
		wantIntervalDays int
		wantRepetitions  int
		wantEaseFactor   float64
		wantNextDueAt    time.Time
		wantAgainStreak  int
		wantLastReviewAt time.Time
	}{
		{
			name: "again returns card to learning",
			current: domain.ReviewState{
				VocabItemID:      "voc_1",
				UserID:           "usr_1",
				Status:           domain.ReviewStatusReview,
				EaseFactor:       2.5,
				IntervalDays:     3,
				RepetitionCount:  2,
				ConsecutiveAgain: 1,
				LastReviewedAt:   &previousReview,
			},
			grade:            domain.ReviewGradeAgain,
			wantStatus:       domain.ReviewStatusLearning,
			wantIntervalDays: 0,
			wantRepetitions:  0,
			wantEaseFactor:   2.3,
			wantNextDueAt:    reviewedAt.Add(4 * time.Hour),
			wantAgainStreak:  2,
			wantLastReviewAt: reviewedAt,
		},
		{
			name: "good second review schedules three days",
			current: domain.ReviewState{
				VocabItemID:     "voc_1",
				UserID:          "usr_1",
				Status:          domain.ReviewStatusReview,
				EaseFactor:      2.5,
				IntervalDays:    1,
				RepetitionCount: 1,
			},
			grade:            domain.ReviewGradeGood,
			wantStatus:       domain.ReviewStatusReview,
			wantIntervalDays: 3,
			wantRepetitions:  2,
			wantEaseFactor:   2.5,
			wantNextDueAt:    reviewedAt.Add(3 * 24 * time.Hour),
			wantLastReviewAt: reviewedAt,
		},
		{
			name: "easy first review increases ease and schedules three days",
			current: domain.ReviewState{
				VocabItemID:     "voc_1",
				UserID:          "usr_1",
				Status:          domain.ReviewStatusNew,
				EaseFactor:      2.5,
				IntervalDays:    0,
				RepetitionCount: 0,
			},
			grade:            domain.ReviewGradeEasy,
			wantStatus:       domain.ReviewStatusReview,
			wantIntervalDays: 3,
			wantRepetitions:  1,
			wantEaseFactor:   2.65,
			wantNextDueAt:    reviewedAt.Add(3 * 24 * time.Hour),
			wantLastReviewAt: reviewedAt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ScheduleReview(tt.current, tt.grade, reviewedAt)
			if err != nil {
				t.Fatalf("schedule review: %v", err)
			}

			if got.Status != tt.wantStatus {
				t.Fatalf("status: got %q want %q", got.Status, tt.wantStatus)
			}
			if got.IntervalDays != tt.wantIntervalDays {
				t.Fatalf("interval days: got %d want %d", got.IntervalDays, tt.wantIntervalDays)
			}
			if got.RepetitionCount != tt.wantRepetitions {
				t.Fatalf("repetition count: got %d want %d", got.RepetitionCount, tt.wantRepetitions)
			}
			if got.EaseFactor != tt.wantEaseFactor {
				t.Fatalf("ease factor: got %v want %v", got.EaseFactor, tt.wantEaseFactor)
			}
			if !got.NextDueAt.Equal(tt.wantNextDueAt) {
				t.Fatalf("next due at: got %s want %s", got.NextDueAt, tt.wantNextDueAt)
			}
			if got.ConsecutiveAgain != tt.wantAgainStreak {
				t.Fatalf("consecutive again: got %d want %d", got.ConsecutiveAgain, tt.wantAgainStreak)
			}
			if got.LastReviewedAt == nil || !got.LastReviewedAt.Equal(tt.wantLastReviewAt) {
				t.Fatalf("last reviewed at: got %v want %s", got.LastReviewedAt, tt.wantLastReviewAt)
			}
		})
	}
}

func TestScheduleReviewRejectsInvalidGrade(t *testing.T) {
	_, err := ScheduleReview(domain.ReviewState{EaseFactor: 2.5}, domain.ReviewGrade("later"), time.Now())
	if err == nil {
		t.Fatal("expected invalid grade error")
	}
}

func TestScheduleReviewKeepsMinimumEaseFactor(t *testing.T) {
	reviewedAt := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	got, err := ScheduleReview(domain.ReviewState{
		EaseFactor:      1.35,
		IntervalDays:    1,
		RepetitionCount: 1,
	}, domain.ReviewGradeAgain, reviewedAt)
	if err != nil {
		t.Fatalf("schedule review: %v", err)
	}

	if got.EaseFactor != 1.3 {
		t.Fatalf("ease factor: got %v want 1.3", got.EaseFactor)
	}
}
