package service

import (
	"errors"
	"math"
	"strings"
	"time"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
)

type App struct {
	store *repository.Store
	clock clock.Clock
}

func NewApp(store *repository.Store, appClock clock.Clock) *App {
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
	if err := a.store.PutMagicLink(token); err != nil {
		return nil, err
	}

	return map[string]string{
		"token":            token.Token,
		"verification_url": strings.TrimRight(baseURL, "/") + "/?token=" + token.Token,
		"expires_at":       token.ExpiresAt.Format(time.RFC3339),
	}, nil
}

func (a *App) VerifyMagicLink(token string) (AuthResult, error) {
	record, ok := a.store.GetMagicLink(token)
	if !ok {
		return AuthResult{}, errors.New("invalid token")
	}
	if a.clock.Now().After(record.ExpiresAt) {
		return AuthResult{}, errors.New("token expired")
	}

	user, ok := a.store.FindUserByEmail(record.Email)
	if !ok {
		user = domain.User{
			ID:        newID("usr"),
			Email:     record.Email,
			CreatedAt: a.clock.Now(),
		}
		if err := a.store.UpsertUser(user); err != nil {
			return AuthResult{}, err
		}
	}

	session := domain.Session{
		Token:     newID("sess"),
		UserID:    user.ID,
		CreatedAt: a.clock.Now(),
		ExpiresAt: a.clock.Now().Add(30 * 24 * time.Hour),
	}
	if err := a.store.PutSession(session); err != nil {
		return AuthResult{}, err
	}
	if err := a.store.DeleteMagicLink(token); err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		User:        user,
		Session:     session,
		RedirectURL: "/auth/success?session_token=" + session.Token,
	}, nil
}

func (a *App) Session(token string) (domain.Session, domain.User, error) {
	session, ok := a.store.GetSession(token)
	if !ok {
		return domain.Session{}, domain.User{}, errors.New("unauthorized")
	}
	if session.ExpiresAt.Before(a.clock.Now()) {
		return domain.Session{}, domain.User{}, errors.New("session expired")
	}
	user, ok := a.store.Users[session.UserID]
	if !ok {
		return domain.Session{}, domain.User{}, errors.New("user not found")
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
	if err := a.store.PutVocab(item, state); err != nil {
		return domain.VocabItem{}, domain.ReviewState{}, err
	}
	if _, err := a.ScheduleNotifications(userID); err != nil {
		return domain.VocabItem{}, domain.ReviewState{}, err
	}
	return item, state, nil
}

func (a *App) UpdateVocab(userID, id string, input CreateVocabInput) (domain.VocabItem, error) {
	item, ok := a.store.GetVocab(id)
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
	if err := a.store.UpdateVocab(item); err != nil {
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
	items := a.store.ListVocabByUser(userID)
	result := make([]VocabWithState, 0, len(items))
	for _, item := range items {
		state, _ := a.store.GetReviewState(item.ID)
		result = append(result, VocabWithState{Item: item, State: state})
	}
	return result, nil
}

type DueCard struct {
	Item  domain.VocabItem   `json:"item"`
	State domain.ReviewState `json:"state"`
}

func (a *App) DueCards(userID string) ([]DueCard, error) {
	states := a.store.ListDueStates(userID, a.clock.Now())
	result := make([]DueCard, 0, len(states))
	for _, state := range states {
		item, ok := a.store.GetVocab(state.VocabItemID)
		if !ok || item.ArchivedAt != nil {
			continue
		}
		result = append(result, DueCard{Item: item, State: state})
	}
	return result, nil
}

func (a *App) GradeReview(userID, vocabID string, grade domain.ReviewGrade) (domain.ReviewState, error) {
	state, ok := a.store.GetReviewState(vocabID)
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
	if err := a.store.PutReviewResult(next, log); err != nil {
		return domain.ReviewState{}, err
	}
	if _, err := a.ScheduleNotifications(userID); err != nil {
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
	item, state, err := a.CreateVocab(userID, CreateVocabInput{
		Term:            input.Term,
		Kind:            domain.CardKindPhrase,
		Meaning:         input.Meaning,
		ExampleSentence: input.ExampleSentence,
		SourceText:      input.Selection,
		SourceURL:       input.PageURL,
		Notes:           input.Notes,
	})
	if err != nil {
		return DueCard{}, err
	}

	capture := domain.CaptureSource{
		ID:          newID("cap"),
		UserID:      userID,
		VocabItemID: item.ID,
		Source:      "chrome-extension",
		Selection:   input.Selection,
		PageTitle:   input.PageTitle,
		PageURL:     input.PageURL,
		CreatedAt:   a.clock.Now(),
	}
	if err := a.store.PutCapture(capture); err != nil {
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
	if err := a.store.UpsertDeviceToken(device); err != nil {
		return domain.DeviceToken{}, err
	}
	return device, nil
}

func (a *App) ScheduleNotifications(userID string) ([]domain.NotificationJob, error) {
	states := a.store.ListDueStates(userID, a.clock.Now())
	jobs := make([]domain.NotificationJob, 0)
	for _, state := range states {
		if _, exists := a.store.FindPendingJob(userID, state.VocabItemID); exists {
			continue
		}
		job := domain.NotificationJob{
			ID:          newID("job"),
			UserID:      userID,
			VocabItemID: state.VocabItemID,
			ScheduledAt: a.clock.Now(),
			Status:      "pending",
			Message:     "Time to review your vocabulary.",
		}
		if err := a.store.PutNotificationJob(job); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (a *App) ListNotificationJobs(userID string) []domain.NotificationJob {
	return a.store.ListNotificationJobs(userID)
}
