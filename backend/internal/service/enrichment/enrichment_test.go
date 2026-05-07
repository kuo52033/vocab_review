package enrichment

import (
	"context"
	"errors"
	"testing"

	"vocabreview/backend/internal/domain"
)

type fakeProvider struct {
	results []Suggestion
	err     error
	items   []Item
}

func (f *fakeProvider) Complete(ctx context.Context, items []Item) ([]Suggestion, error) {
	f.items = append([]Item(nil), items...)
	return f.results, f.err
}

func TestEnricherPreservesOrderAndFillsMissingFields(t *testing.T) {
	provider := &fakeProvider{results: []Suggestion{
		{Term: "serendipity", Meaning: "happy accident", ExampleSentence: "It was serendipity.", PartOfSpeech: domain.PartOfSpeechNoun},
		{Term: "meticulous", Meaning: "careful", ExampleSentence: "She was meticulous.", PartOfSpeech: domain.PartOfSpeechAdjective},
	}}
	enricher := New(provider, 20)

	result, err := enricher.Autocomplete(context.Background(), []Item{
		{Term: "serendipity"},
		{Term: "meticulous", Meaning: "already careful"},
	})
	if err != nil {
		t.Fatalf("autocomplete: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("result length: got %d want 2", len(result))
	}
	if result[0].Meaning != "happy accident" || result[0].PartOfSpeech != domain.PartOfSpeechNoun {
		t.Fatalf("unexpected first result: %+v", result[0])
	}
	if result[1].Meaning != "already careful" || result[1].ExampleSentence != "She was meticulous." {
		t.Fatalf("unexpected second result: %+v", result[1])
	}
}

func TestEnricherValidatesBatch(t *testing.T) {
	enricher := New(&fakeProvider{}, 1)
	if _, err := enricher.Autocomplete(context.Background(), nil); !errors.Is(err, ErrEmptyBatch) {
		t.Fatalf("empty batch error: got %v want %v", err, ErrEmptyBatch)
	}
	if _, err := enricher.Autocomplete(context.Background(), []Item{{Term: "one"}, {Term: "two"}}); !errors.Is(err, ErrBatchTooLarge) {
		t.Fatalf("large batch error: got %v want %v", err, ErrBatchTooLarge)
	}
	if _, err := enricher.Autocomplete(context.Background(), []Item{{Term: " "}}); !errors.Is(err, ErrTermRequired) {
		t.Fatalf("term required error: got %v want %v", err, ErrTermRequired)
	}
}

func TestEnricherReportsProviderFailure(t *testing.T) {
	enricher := New(&fakeProvider{err: errors.New("provider down")}, 20)
	_, err := enricher.Autocomplete(context.Background(), []Item{{Term: "serendipity"}})
	if !errors.Is(err, ErrProviderFailed) {
		t.Fatalf("provider error: got %v want %v", err, ErrProviderFailed)
	}
}
