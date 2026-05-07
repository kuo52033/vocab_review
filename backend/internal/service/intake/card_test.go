package intake

import (
	"testing"
	"time"

	"vocabreview/backend/internal/domain"
)

func TestNewVocabCardBuildsInitialDueCard(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)

	card, err := NewVocabCard("usr_1", VocabInput{
		Term:            "  serendipity  ",
		Kind:            "",
		Meaning:         "  happy accident  ",
		ExampleSentence: "  That was serendipity.  ",
		SourceText:      "  source text  ",
		SourceURL:       "  https://example.com  ",
		Notes:           "  note  ",
	}, IDs{VocabItemID: "voc_1", NotificationJobID: "job_1"}, now)
	if err != nil {
		t.Fatalf("new vocab card: %v", err)
	}

	if card.Item.ID != "voc_1" || card.Item.UserID != "usr_1" {
		t.Fatalf("unexpected item identity: %+v", card.Item)
	}
	if card.Item.Term != "serendipity" || card.Item.Kind != domain.CardKindWord || card.Item.Meaning != "happy accident" {
		t.Fatalf("unexpected normalized item: %+v", card.Item)
	}
	if card.Item.ExampleSentence != "That was serendipity." || card.Item.SourceText != "source text" || card.Item.SourceURL != "https://example.com" || card.Item.Notes != "note" {
		t.Fatalf("unexpected trimmed fields: %+v", card.Item)
	}
	if !card.Item.CreatedAt.Equal(now) || !card.Item.UpdatedAt.Equal(now) {
		t.Fatalf("unexpected timestamps: %+v", card.Item)
	}

	if card.State.VocabItemID != "voc_1" || card.State.UserID != "usr_1" {
		t.Fatalf("unexpected state identity: %+v", card.State)
	}
	if card.State.Status != domain.ReviewStatusNew || card.State.EaseFactor != 2.5 || card.State.IntervalDays != 0 || card.State.RepetitionCount != 0 || !card.State.NextDueAt.Equal(now) {
		t.Fatalf("unexpected initial state: %+v", card.State)
	}

	if card.NotificationJob == nil {
		t.Fatal("expected pending notification job")
	}
	if card.NotificationJob.ID != "job_1" || card.NotificationJob.UserID != "usr_1" || card.NotificationJob.VocabItemID != "voc_1" || card.NotificationJob.Status != "pending" {
		t.Fatalf("unexpected notification job: %+v", card.NotificationJob)
	}
}

func TestNewVocabCardRequiresTerm(t *testing.T) {
	_, err := NewVocabCard("usr_1", VocabInput{Term: "  "}, IDs{}, time.Now())
	if err == nil {
		t.Fatal("expected term required error")
	}
}

func TestNewCapturedCardBuildsCaptureSource(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)

	card, err := NewCapturedCard("usr_1", CaptureInput{
		Term:            " phrase ",
		Meaning:         " meaning ",
		ExampleSentence: " example ",
		Selection:       " selected text ",
		PageTitle:       "Page title",
		PageURL:         "https://example.com/page",
		Notes:           " note ",
	}, IDs{VocabItemID: "voc_1", CaptureSourceID: "cap_1", NotificationJobID: "job_1"}, now)
	if err != nil {
		t.Fatalf("new captured card: %v", err)
	}

	if card.Item.Kind != domain.CardKindPhrase || card.Item.SourceText != "selected text" || card.Item.SourceURL != "https://example.com/page" {
		t.Fatalf("unexpected captured item: %+v", card.Item)
	}
	if card.Capture.ID != "cap_1" || card.Capture.UserID != "usr_1" || card.Capture.VocabItemID != "voc_1" {
		t.Fatalf("unexpected capture identity: %+v", card.Capture)
	}
	if card.Capture.Source != "chrome-extension" || card.Capture.Selection != " selected text " || card.Capture.PageTitle != "Page title" || card.Capture.PageURL != "https://example.com/page" {
		t.Fatalf("unexpected capture source: %+v", card.Capture)
	}
	if card.NotificationJob == nil || card.NotificationJob.ID != "job_1" {
		t.Fatalf("unexpected notification job: %+v", card.NotificationJob)
	}
}
