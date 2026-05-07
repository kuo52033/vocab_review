package enrichment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"vocabreview/backend/internal/domain"
)

var (
	ErrEmptyBatch     = errors.New("empty enrichment batch")
	ErrBatchTooLarge  = errors.New("enrichment batch too large")
	ErrTermRequired   = errors.New("term is required")
	ErrProviderFailed = errors.New("enrichment provider failed")
)

type Item struct {
	Term            string              `json:"term"`
	Meaning         string              `json:"meaning"`
	ExampleSentence string              `json:"example_sentence"`
	PartOfSpeech    domain.PartOfSpeech `json:"part_of_speech"`
}

type Suggestion struct {
	Term            string              `json:"term"`
	Meaning         string              `json:"meaning"`
	ExampleSentence string              `json:"example_sentence"`
	PartOfSpeech    domain.PartOfSpeech `json:"part_of_speech"`
	Error           string              `json:"error"`
}

type Provider interface {
	Complete(ctx context.Context, items []Item) ([]Suggestion, error)
}

type Enricher struct {
	provider Provider
	maxBatch int
}

func New(provider Provider, maxBatch int) *Enricher {
	if maxBatch <= 0 {
		maxBatch = 20
	}
	return &Enricher{
		provider: provider,
		maxBatch: maxBatch,
	}
}

func (e *Enricher) Autocomplete(ctx context.Context, items []Item) ([]Suggestion, error) {
	normalized, err := e.validateItems(items)
	if err != nil {
		return nil, err
	}

	suggestions, err := e.provider.Complete(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderFailed, err)
	}
	if len(suggestions) != len(normalized) {
		return nil, fmt.Errorf("%w: provider returned %d suggestions for %d items", ErrProviderFailed, len(suggestions), len(normalized))
	}

	result := make([]Suggestion, len(normalized))
	for i, item := range normalized {
		result[i] = fillMissingFields(item, suggestions[i])
	}
	return result, nil
}

func (e *Enricher) validateItems(items []Item) ([]Item, error) {
	if len(items) == 0 {
		return nil, ErrEmptyBatch
	}
	if len(items) > e.maxBatch {
		return nil, ErrBatchTooLarge
	}

	normalized := make([]Item, len(items))
	for i, item := range items {
		normalized[i] = Item{
			Term:            strings.TrimSpace(item.Term),
			Meaning:         strings.TrimSpace(item.Meaning),
			ExampleSentence: strings.TrimSpace(item.ExampleSentence),
			PartOfSpeech:    item.PartOfSpeech,
		}
		if normalized[i].Term == "" {
			return nil, ErrTermRequired
		}
	}
	return normalized, nil
}

func fillMissingFields(item Item, suggestion Suggestion) Suggestion {
	result := Suggestion{
		Term:            strings.TrimSpace(suggestion.Term),
		Meaning:         strings.TrimSpace(suggestion.Meaning),
		ExampleSentence: strings.TrimSpace(suggestion.ExampleSentence),
		PartOfSpeech:    suggestion.PartOfSpeech,
		Error:           suggestion.Error,
	}
	if result.Term == "" {
		result.Term = item.Term
	}
	if item.Meaning != "" {
		result.Meaning = item.Meaning
	}
	if item.ExampleSentence != "" {
		result.ExampleSentence = item.ExampleSentence
	}
	if item.PartOfSpeech != domain.PartOfSpeechUnspecified {
		result.PartOfSpeech = item.PartOfSpeech
	}
	return result
}
