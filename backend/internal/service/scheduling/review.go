package scheduling

import (
	"errors"
	"math"
	"time"

	"vocabreview/backend/internal/domain"
)

func ScheduleReview(current domain.ReviewState, grade domain.ReviewGrade, reviewedAt time.Time) (domain.ReviewState, error) {
	next := current
	next.LastReviewedAt = &reviewedAt

	switch grade {
	case domain.ReviewGradeAgain:
		next.Status = domain.ReviewStatusLearning
		next.IntervalDays = 0
		next.RepetitionCount = 0
		next.ConsecutiveAgain++
		next.EaseFactor = math.Max(1.3, next.EaseFactor-0.2)
		next.NextDueAt = reviewedAt.Add(4 * time.Hour)
	case domain.ReviewGradeHard:
		next.Status = domain.ReviewStatusReview
		next.ConsecutiveAgain = 0
		next.RepetitionCount++
		if next.RepetitionCount <= 1 {
			next.IntervalDays = 1
		} else {
			next.IntervalDays = max(1, int(math.Round(float64(max(1, current.IntervalDays))*1.2)))
		}
		next.EaseFactor = math.Max(1.3, next.EaseFactor-0.15)
		next.NextDueAt = reviewedAt.Add(time.Duration(next.IntervalDays) * 24 * time.Hour)
	case domain.ReviewGradeGood:
		next.Status = domain.ReviewStatusReview
		next.ConsecutiveAgain = 0
		next.RepetitionCount++
		if next.RepetitionCount == 1 {
			next.IntervalDays = 1
		} else if next.RepetitionCount == 2 {
			next.IntervalDays = 3
		} else {
			next.IntervalDays = max(1, int(math.Round(float64(max(1, current.IntervalDays))*current.EaseFactor)))
		}
		next.NextDueAt = reviewedAt.Add(time.Duration(next.IntervalDays) * 24 * time.Hour)
	case domain.ReviewGradeEasy:
		next.Status = domain.ReviewStatusReview
		next.ConsecutiveAgain = 0
		next.RepetitionCount++
		next.EaseFactor += 0.15
		if next.RepetitionCount == 1 {
			next.IntervalDays = 3
		} else {
			next.IntervalDays = max(2, int(math.Round(float64(max(1, current.IntervalDays))*(current.EaseFactor+0.3))))
		}
		next.NextDueAt = reviewedAt.Add(time.Duration(next.IntervalDays) * 24 * time.Hour)
	default:
		return domain.ReviewState{}, errors.New("invalid grade")
	}

	return next, nil
}
