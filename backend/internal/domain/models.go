package domain

import "time"

type CardKind string

const (
	CardKindWord   CardKind = "word"
	CardKindPhrase CardKind = "phrase"
)

type PartOfSpeech string

const (
	PartOfSpeechUnspecified  PartOfSpeech = ""
	PartOfSpeechNoun         PartOfSpeech = "noun"
	PartOfSpeechVerb         PartOfSpeech = "verb"
	PartOfSpeechAdjective    PartOfSpeech = "adjective"
	PartOfSpeechAdverb       PartOfSpeech = "adverb"
	PartOfSpeechPhrase       PartOfSpeech = "phrase"
	PartOfSpeechIdiom        PartOfSpeech = "idiom"
	PartOfSpeechPhrasalVerb  PartOfSpeech = "phrasal_verb"
	PartOfSpeechPreposition  PartOfSpeech = "preposition"
	PartOfSpeechConjunction  PartOfSpeech = "conjunction"
	PartOfSpeechInterjection PartOfSpeech = "interjection"
	PartOfSpeechDeterminer   PartOfSpeech = "determiner"
	PartOfSpeechPronoun      PartOfSpeech = "pronoun"
	PartOfSpeechOther        PartOfSpeech = "other"
)

type ReviewGrade string

const (
	ReviewGradeAgain ReviewGrade = "again"
	ReviewGradeHard  ReviewGrade = "hard"
	ReviewGradeGood  ReviewGrade = "good"
	ReviewGradeEasy  ReviewGrade = "easy"
)

type ReviewStatus string

const (
	ReviewStatusNew      ReviewStatus = "new"
	ReviewStatusLearning ReviewStatus = "learning"
	ReviewStatusReview   ReviewStatus = "review"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type MagicLinkToken struct {
	Token     string    `json:"token"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
}

type VocabItem struct {
	ID              string       `json:"id"`
	UserID          string       `json:"user_id"`
	Term            string       `json:"term"`
	Kind            CardKind     `json:"kind"`
	Meaning         string       `json:"meaning"`
	ExampleSentence string       `json:"example_sentence"`
	PartOfSpeech    PartOfSpeech `json:"part_of_speech"`
	SourceText      string       `json:"source_text"`
	SourceURL       string       `json:"source_url"`
	Notes           string       `json:"notes"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
	ArchivedAt      *time.Time   `json:"archived_at,omitempty"`
}

type ReviewState struct {
	VocabItemID      string       `json:"vocab_item_id"`
	UserID           string       `json:"user_id"`
	Status           ReviewStatus `json:"status"`
	EaseFactor       float64      `json:"ease_factor"`
	IntervalDays     int          `json:"interval_days"`
	RepetitionCount  int          `json:"repetition_count"`
	LastReviewedAt   *time.Time   `json:"last_reviewed_at,omitempty"`
	NextDueAt        time.Time    `json:"next_due_at"`
	ConsecutiveAgain int          `json:"consecutive_again"`
}

type ReviewLog struct {
	ID          string      `json:"id"`
	UserID      string      `json:"user_id"`
	VocabItemID string      `json:"vocab_item_id"`
	Grade       ReviewGrade `json:"grade"`
	ReviewedAt  time.Time   `json:"reviewed_at"`
}

type CaptureSource struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	VocabItemID string    `json:"vocab_item_id"`
	Source      string    `json:"source"`
	Selection   string    `json:"selection"`
	PageTitle   string    `json:"page_title"`
	PageURL     string    `json:"page_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type DeviceToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Platform  string    `json:"platform"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type NotificationJob struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	VocabItemID string     `json:"vocab_item_id"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
	Status      string     `json:"status"`
	Message     string     `json:"message"`
}
