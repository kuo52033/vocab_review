# Vocab Autocomplete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add on-demand AI-assisted bulk autocomplete for missing vocab meaning, example sentence, and part of speech in the Web Bulk Import flow.

**Architecture:** Add an optional backend enrichment seam that calls an OpenAI-compatible provider once per batch and returns suggestions without saving cards. Add `part_of_speech` to the persisted vocab model, then let the web bulk-import UI enrich parsed rows and fill empty fields only.

**Tech Stack:** Go `net/http`, pgx/Postgres, goose SQL migrations, Go `testing`, React + TypeScript + Vite.

---

## File Structure

- Create `backend/migrations/00002_add_part_of_speech.sql`: adds strict optional `part_of_speech`.
- Modify `backend/internal/domain/models.go`: adds `PartOfSpeech` value and allowed constants.
- Modify `backend/internal/repository/postgres/{tx.go,scans.go,vocab.go,store_integration_test.go}`: persists and validates the new column.
- Modify `backend/internal/service/intake/{card.go,card_test.go}`: normalizes `part_of_speech` during card creation.
- Create `backend/internal/service/enrichment/{enrichment.go,enrichment_test.go,openai.go,openai_test.go}`: batch enrichment seam and OpenAI-compatible adapter.
- Modify `backend/internal/service/review.go`: exposes `AutocompleteVocab` through `service.App`.
- Modify `backend/internal/httpapi/{server.go,vocab.go}`: authenticated `POST /vocab/autocomplete`.
- Modify `backend/cmd/api/main.go`: builds optional enrichment provider from environment.
- Modify `.env.example`: documents enrichment configuration.
- Modify `apps/web/src/{api.ts,App.tsx,styles.css}`: bulk import autocomplete UI.

---

### Task 1: Persist `part_of_speech`

**Files:**
- Create: `backend/migrations/00002_add_part_of_speech.sql`
- Modify: `backend/internal/domain/models.go`
- Modify: `backend/internal/repository/postgres/tx.go`
- Modify: `backend/internal/repository/postgres/scans.go`
- Modify: `backend/internal/repository/postgres/vocab.go`
- Modify: `backend/internal/repository/postgres/store_integration_test.go`
- Modify: `backend/internal/service/intake/card.go`
- Modify: `backend/internal/service/intake/card_test.go`

- [ ] **Step 1: Write failing repository test**

Add to `backend/internal/repository/postgres/store_integration_test.go`:

```go
func TestStorePersistsPartOfSpeech(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for postgres integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resetDatabase(t, databaseURL)

	store, err := New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	user := domain.User{ID: "usr_pos", Email: "pos@example.com", CreatedAt: now}
	if _, err := store.pool.Exec(ctx, `INSERT INTO users (id, email, created_at) VALUES ($1, $2, $3)`, user.ID, user.Email, user.CreatedAt); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	item := domain.VocabItem{
		ID:              "voc_pos",
		UserID:          user.ID,
		Term:            "serendipity",
		Kind:            domain.CardKindWord,
		Meaning:         "happy accident",
		ExampleSentence: "It was serendipity.",
		PartOfSpeech:    domain.PartOfSpeechNoun,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	state := domain.ReviewState{
		VocabItemID:     item.ID,
		UserID:          user.ID,
		Status:          domain.ReviewStatusNew,
		EaseFactor:      2.5,
		IntervalDays:    0,
		RepetitionCount: 0,
		NextDueAt:       now,
	}
	if err := store.CreateVocab(ctx, item, state, nil); err != nil {
		t.Fatalf("create vocab: %v", err)
	}

	loaded, ok, err := store.GetVocab(ctx, item.ID)
	if err != nil {
		t.Fatalf("get vocab: %v", err)
	}
	if !ok {
		t.Fatal("expected vocab")
	}
	if loaded.PartOfSpeech != domain.PartOfSpeechNoun {
		t.Fatalf("part of speech: got %q want %q", loaded.PartOfSpeech, domain.PartOfSpeechNoun)
	}

	item.ID = "voc_invalid_pos"
	item.PartOfSpeech = domain.PartOfSpeech("invalid")
	err = store.CreateVocab(ctx, item, domain.ReviewState{
		VocabItemID:     item.ID,
		UserID:          user.ID,
		Status:          domain.ReviewStatusNew,
		EaseFactor:      2.5,
		IntervalDays:    0,
		RepetitionCount: 0,
		NextDueAt:       now,
	}, nil)
	if err == nil {
		t.Fatal("expected invalid part_of_speech constraint error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd backend
go test ./internal/repository/postgres -run TestStorePersistsPartOfSpeech -count=1
```

Expected: FAIL because `domain.VocabItem` has no `PartOfSpeech`.

- [ ] **Step 3: Add domain type and migration**

In `backend/internal/domain/models.go`, add:

```go
type PartOfSpeech string

const (
	PartOfSpeechNone        PartOfSpeech = ""
	PartOfSpeechNoun        PartOfSpeech = "noun"
	PartOfSpeechVerb        PartOfSpeech = "verb"
	PartOfSpeechAdjective   PartOfSpeech = "adjective"
	PartOfSpeechAdverb      PartOfSpeech = "adverb"
	PartOfSpeechPhrase      PartOfSpeech = "phrase"
	PartOfSpeechIdiom       PartOfSpeech = "idiom"
	PartOfSpeechPhrasalVerb PartOfSpeech = "phrasal_verb"
	PartOfSpeechPreposition PartOfSpeech = "preposition"
	PartOfSpeechConjunction PartOfSpeech = "conjunction"
	PartOfSpeechInterjection PartOfSpeech = "interjection"
	PartOfSpeechDeterminer  PartOfSpeech = "determiner"
	PartOfSpeechPronoun     PartOfSpeech = "pronoun"
	PartOfSpeechOther       PartOfSpeech = "other"
)
```

Add to `VocabItem` after `ExampleSentence`:

```go
PartOfSpeech PartOfSpeech `json:"part_of_speech"`
```

Create `backend/migrations/00002_add_part_of_speech.sql`:

```sql
-- +goose Up
ALTER TABLE vocab_items
ADD COLUMN part_of_speech TEXT NOT NULL DEFAULT '';

ALTER TABLE vocab_items
ADD CONSTRAINT vocab_items_part_of_speech_check
CHECK (
    part_of_speech IN (
        '',
        'noun',
        'verb',
        'adjective',
        'adverb',
        'phrase',
        'idiom',
        'phrasal_verb',
        'preposition',
        'conjunction',
        'interjection',
        'determiner',
        'pronoun',
        'other'
    )
);

-- +goose Down
ALTER TABLE vocab_items
DROP CONSTRAINT vocab_items_part_of_speech_check;

ALTER TABLE vocab_items
DROP COLUMN part_of_speech;
```

- [ ] **Step 4: Update Postgres scans and writes**

In `backend/internal/repository/postgres/tx.go`, update `insertVocab` column and value lists to include `part_of_speech` after `example_sentence`.

```sql
id, user_id, term, kind, meaning, example_sentence, part_of_speech, source_text, source_url, notes, created_at, updated_at, archived_at
```

Pass:

```go
item.PartOfSpeech
```

between `item.ExampleSentence` and `item.SourceText`.

In every vocab `SELECT` / `RETURNING` list in `backend/internal/repository/postgres/vocab.go` and `backend/internal/repository/postgres/review.go`, include `part_of_speech` after `example_sentence`.

In `backend/internal/repository/postgres/scans.go`, add scan targets after `ExampleSentence`:

```go
&item.PartOfSpeech,
```

Do this in both `scanVocab` and `scanVocabWithStates`.

- [ ] **Step 5: Update service intake model**

In `backend/internal/service/intake/card.go`, add to `VocabInput`:

```go
PartOfSpeech domain.PartOfSpeech
```

Set `VocabItem.PartOfSpeech`:

```go
PartOfSpeech: input.PartOfSpeech,
```

In `NewCapturedCard`, pass through an empty part of speech:

```go
PartOfSpeech: "",
```

In `backend/internal/service/review.go`, add `PartOfSpeech domain.PartOfSpeech` to `CreateVocabInput` and pass it into `intake.VocabInput`.

- [ ] **Step 6: Run tests**

Run:

```bash
cd backend
gofmt -w internal
go test ./...
```

Expected: all Go tests pass.

- [ ] **Step 7: Commit**

```bash
git add backend/migrations/00002_add_part_of_speech.sql backend/internal/domain/models.go backend/internal/repository/postgres backend/internal/service/intake backend/internal/service/review.go
git commit -m "feat: add part of speech to vocab cards"
```

---

### Task 2: Add Enrichment Service

**Files:**
- Create: `backend/internal/service/enrichment/enrichment.go`
- Create: `backend/internal/service/enrichment/enrichment_test.go`

- [ ] **Step 1: Write failing enrichment service tests**

Create `backend/internal/service/enrichment/enrichment_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd backend
go test ./internal/service/enrichment
```

Expected: FAIL because the package implementation does not exist.

- [ ] **Step 3: Implement enrichment service**

Create `backend/internal/service/enrichment/enrichment.go`:

```go
package enrichment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"vocabreview/backend/internal/domain"
)

var (
	ErrEmptyBatch     = errors.New("empty autocomplete batch")
	ErrBatchTooLarge  = errors.New("autocomplete batch too large")
	ErrTermRequired   = errors.New("term is required")
	ErrProviderFailed = errors.New("vocab enrichment provider failed")
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
	return &Enricher{provider: provider, maxBatch: maxBatch}
}

func (e *Enricher) Autocomplete(ctx context.Context, items []Item) ([]Suggestion, error) {
	if len(items) == 0 {
		return nil, ErrEmptyBatch
	}
	if len(items) > e.maxBatch {
		return nil, ErrBatchTooLarge
	}
	normalized := make([]Item, len(items))
	for index, item := range items {
		term := strings.TrimSpace(item.Term)
		if term == "" {
			return nil, ErrTermRequired
		}
		item.Term = term
		item.Meaning = strings.TrimSpace(item.Meaning)
		item.ExampleSentence = strings.TrimSpace(item.ExampleSentence)
		normalized[index] = item
	}

	suggestions, err := e.provider.Complete(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProviderFailed, err)
	}
	if len(suggestions) != len(normalized) {
		return nil, fmt.Errorf("%w: expected %d suggestions, got %d", ErrProviderFailed, len(normalized), len(suggestions))
	}

	result := make([]Suggestion, len(normalized))
	for index, item := range normalized {
		suggestion := suggestions[index]
		suggestion.Term = item.Term
		if item.Meaning != "" {
			suggestion.Meaning = item.Meaning
		}
		if item.ExampleSentence != "" {
			suggestion.ExampleSentence = item.ExampleSentence
		}
		if item.PartOfSpeech != "" {
			suggestion.PartOfSpeech = item.PartOfSpeech
		}
		result[index] = suggestion
	}
	return result, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend
gofmt -w internal/service/enrichment
go test ./internal/service/enrichment
go test ./internal/service/...
```

Expected: all service package tests pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/enrichment
git commit -m "feat: add vocab enrichment service"
```

---

### Task 3: Add OpenAI-Compatible Provider Adapter

**Files:**
- Create: `backend/internal/service/enrichment/openai.go`
- Create: `backend/internal/service/enrichment/openai_test.go`

- [ ] **Step 1: Write failing adapter tests**

Create `backend/internal/service/enrichment/openai_test.go`:

```go
package enrichment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vocabreview/backend/internal/domain"
)

func TestOpenAIProviderCompletesBatch(t *testing.T) {
	var authHeader string
	var requestedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		requestedPath = r.URL.Path
		var req openAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Fatalf("model: got %q want test-model", req.Model)
		}
		_ = json.NewEncoder(w).Encode(openAIChatResponse{
			Choices: []openAIChoice{{
				Message: openAIMessage{Content: `{"items":[{"term":"serendipity","meaning":"happy accident","example_sentence":"It was serendipity.","part_of_speech":"noun","error":""}]}`},
			}},
		})
	}))
	defer server.Close()

	provider := NewOpenAIProvider(server.URL, "secret", "test-model", server.Client())
	result, err := provider.Complete(context.Background(), []Item{{Term: "serendipity"}})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if requestedPath != "/chat/completions" {
		t.Fatalf("path: got %q want /chat/completions", requestedPath)
	}
	if authHeader != "Bearer secret" {
		t.Fatalf("auth header: got %q", authHeader)
	}
	if len(result) != 1 || result[0].Meaning != "happy accident" || result[0].PartOfSpeech != domain.PartOfSpeechNoun {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenAIProviderRejectsInvalidJSONContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(openAIChatResponse{
			Choices: []openAIChoice{{Message: openAIMessage{Content: `not-json`}}},
		})
	}))
	defer server.Close()

	provider := NewOpenAIProvider(server.URL, "secret", "test-model", server.Client())
	if _, err := provider.Complete(context.Background(), []Item{{Term: "serendipity"}}); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd backend
go test ./internal/service/enrichment -run OpenAI
```

Expected: FAIL because `NewOpenAIProvider` and OpenAI request/response types do not exist.

- [ ] **Step 3: Implement adapter**

Create `backend/internal/service/enrichment/openai.go` with these exported and unexported types:

```go
package enrichment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type OpenAIProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

type openAIChatRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []openAIChoice `json:"choices"`
}

type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

func NewOpenAIProvider(baseURL string, apiKey string, model string, client *http.Client) *OpenAIProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenAIProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		client:  client,
	}
}

func (p *OpenAIProvider) Complete(ctx context.Context, items []Item) ([]Suggestion, error) {
	payload, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	reqBody := openAIChatRequest{
		Model: p.model,
		Messages: []openAIMessage{
			{Role: "system", Content: "You enrich English vocabulary cards. Return strict JSON only. Use part_of_speech values from this list: noun, verb, adjective, adverb, phrase, idiom, phrasal_verb, preposition, conjunction, interjection, determiner, pronoun, other."},
			{Role: "user", Content: "Return JSON as {\"items\":[{\"term\":\"\",\"meaning\":\"\",\"example_sentence\":\"\",\"part_of_speech\":\"\",\"error\":\"\"}]}. Preserve item order. Fill missing details for these items: " + string(payload)},
		},
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider status %d", resp.StatusCode)
	}

	var chat openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chat); err != nil {
		return nil, err
	}
	if len(chat.Choices) == 0 {
		return nil, fmt.Errorf("provider returned no choices")
	}
	var parsed struct {
		Items []Suggestion `json:"items"`
	}
	if err := json.Unmarshal([]byte(chat.Choices[0].Message.Content), &parsed); err != nil {
		return nil, err
	}
	return parsed.Items, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend
gofmt -w internal/service/enrichment
go test ./internal/service/enrichment
```

Expected: enrichment tests pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/enrichment
git commit -m "feat: add openai compatible enrichment provider"
```

---

### Task 4: Add Backend Endpoint and Config Wiring

**Files:**
- Modify: `backend/internal/service/review.go`
- Create: `backend/internal/httpapi/vocab_autocomplete_test.go`
- Modify: `backend/internal/httpapi/server.go`
- Modify: `backend/internal/httpapi/vocab.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `.env.example`

- [ ] **Step 1: Add service tests for unconfigured and fake-enriched autocomplete**

Add to `backend/internal/service/review_test.go`:

```go
type fakeEnricher struct {
	items []enrichment.Item
}

func (f *fakeEnricher) Autocomplete(ctx context.Context, items []enrichment.Item) ([]enrichment.Suggestion, error) {
	f.items = append([]enrichment.Item(nil), items...)
	return []enrichment.Suggestion{{Term: items[0].Term, Meaning: "happy accident", ExampleSentence: "It was serendipity.", PartOfSpeech: domain.PartOfSpeechNoun}}, nil
}

func TestAutocompleteVocabRequiresConfiguredEnricher(t *testing.T) {
	app := newTestApp()
	_, err := app.AutocompleteVocab([]enrichment.Item{{Term: "serendipity"}})
	if !errors.Is(err, ErrEnrichmentNotConfigured) {
		t.Fatalf("error: got %v want %v", err, ErrEnrichmentNotConfigured)
	}
}

func TestAutocompleteVocabUsesConfiguredEnricher(t *testing.T) {
	repo := newFakeRepository()
	fake := &fakeEnricher{}
	app := NewAppWithEnricher(repo, stubClock{now: time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)}, fake)

	result, err := app.AutocompleteVocab([]enrichment.Item{{Term: "serendipity"}})
	if err != nil {
		t.Fatalf("autocomplete: %v", err)
	}
	if len(result) != 1 || result[0].Meaning != "happy accident" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(fake.items) != 1 || fake.items[0].Term != "serendipity" {
		t.Fatalf("unexpected provider input: %+v", fake.items)
	}
}
```

Import `vocabreview/backend/internal/service/enrichment` in the test.

Create `backend/internal/httpapi/vocab_autocomplete_test.go`:

```go
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vocabreview/backend/internal/clock"
	"vocabreview/backend/internal/domain"
	"vocabreview/backend/internal/repository"
	"vocabreview/backend/internal/service"
	"vocabreview/backend/internal/service/enrichment"
)

type autocompleteHTTPRepository struct {
	repository.AppRepository
}

func (autocompleteHTTPRepository) GetSessionUser(_ context.Context, token string) (domain.Session, domain.User, bool, error) {
	if token != "sess_test" {
		return domain.Session{}, domain.User{}, false, nil
	}
	return domain.Session{
		Token:     token,
		UserID:    "usr_test",
		CreatedAt: time.Now().Add(-time.Hour),
		ExpiresAt: time.Now().Add(time.Hour),
	}, domain.User{ID: "usr_test", Email: "test@example.com", CreatedAt: time.Now().Add(-time.Hour)}, true, nil
}

func (autocompleteHTTPRepository) HealthCheck(context.Context) error {
	return nil
}

type autocompleteHTTPEnricher struct {
	err error
}

func (e autocompleteHTTPEnricher) Autocomplete(_ context.Context, items []enrichment.Item) ([]enrichment.Suggestion, error) {
	if e.err != nil {
		return nil, e.err
	}
	return []enrichment.Suggestion{{
		Term:            items[0].Term,
		Meaning:         "happy accident",
		ExampleSentence: "It was serendipity.",
		PartOfSpeech:    domain.PartOfSpeechNoun,
	}}, nil
}

func TestHandleAutocompleteVocabReturnsSuggestions(t *testing.T) {
	app := service.NewAppWithEnricher(autocompleteHTTPRepository{}, clock.RealClock{}, autocompleteHTTPEnricher{})
	server := NewServer(app, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	body := strings.NewReader(`{"items":[{"term":"serendipity"}]}`)
	request := httptest.NewRequest(http.MethodPost, "/vocab/autocomplete", body)
	request.Header.Set("Authorization", "Bearer sess_test")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status: got %d body %s", response.Code, response.Body.String())
	}
	var payload struct {
		Items []enrichment.Suggestion `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].Meaning != "happy accident" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestHandleAutocompleteVocabMapsValidationErrors(t *testing.T) {
	app := service.NewAppWithEnricher(autocompleteHTTPRepository{}, clock.RealClock{}, autocompleteHTTPEnricher{err: enrichment.ErrEmptyBatch})
	server := NewServer(app, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	request := httptest.NewRequest(http.MethodPost, "/vocab/autocomplete", strings.NewReader(`{"items":[]}`))
	request.Header.Set("Authorization", "Bearer sess_test")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body %s", response.Code, response.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
cd backend
go test ./internal/service -run Autocomplete
go test ./internal/httpapi -run Autocomplete
```

Expected: FAIL because `NewAppWithEnricher`, `ErrEnrichmentNotConfigured`, `AutocompleteVocab`, and `handleAutocompleteVocab` do not exist.

- [ ] **Step 3: Add service wiring**

In `backend/internal/service/review.go`, add:

```go
var ErrEnrichmentNotConfigured = errors.New("vocab enrichment is not configured")

type VocabEnricher interface {
	Autocomplete(ctx context.Context, items []enrichment.Item) ([]enrichment.Suggestion, error)
}
```

Update `App`:

```go
type App struct {
	store    repository.AppRepository
	clock    clock.Clock
	enricher VocabEnricher
}
```

Keep `NewApp` and add:

```go
func NewApp(store repository.AppRepository, appClock clock.Clock) *App {
	return NewAppWithEnricher(store, appClock, nil)
}

func NewAppWithEnricher(store repository.AppRepository, appClock clock.Clock, enricher VocabEnricher) *App {
	return &App{store: store, clock: appClock, enricher: enricher}
}

func (a *App) AutocompleteVocab(items []enrichment.Item) ([]enrichment.Suggestion, error) {
	if a.enricher == nil {
		return nil, ErrEnrichmentNotConfigured
	}
	return a.enricher.Autocomplete(context.Background(), items)
}
```

- [ ] **Step 4: Add HTTP route and error mapping**

In `backend/internal/httpapi/server.go`, add route:

```go
s.mux.Handle("POST /vocab/autocomplete", s.requireAuth(http.HandlerFunc(s.handleAutocompleteVocab)))
```

Place it before `s.mux.Handle("POST /vocab", ...)` so the exact route wins clearly.

In `backend/internal/httpapi/vocab.go`, add:

```go
func (s *Server) handleAutocompleteVocab(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []enrichment.Item `json:"items"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := s.app.AutocompleteVocab(req.Items)
	if errors.Is(err, service.ErrEnrichmentNotConfigured) {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if errors.Is(err, enrichment.ErrEmptyBatch) || errors.Is(err, enrichment.ErrBatchTooLarge) || errors.Is(err, enrichment.ErrTermRequired) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, enrichment.ErrProviderFailed) {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
```

Add imports for `errors`, `enrichment`, and `service` as needed.

- [ ] **Step 5: Wire provider config in main**

In `backend/cmd/api/main.go`, import `net/http` if not already available and `vocabreview/backend/internal/service/enrichment`.

Add helper:

```go
func newVocabEnricherFromEnv() service.VocabEnricher {
	baseURL := os.Getenv("VOCAB_ENRICHMENT_BASE_URL")
	apiKey := os.Getenv("VOCAB_ENRICHMENT_API_KEY")
	model := os.Getenv("VOCAB_ENRICHMENT_MODEL")
	if baseURL == "" || apiKey == "" || model == "" {
		return nil
	}
	provider := enrichment.NewOpenAIProvider(baseURL, apiKey, model, http.DefaultClient)
	return enrichment.New(provider, 20)
}
```

Change app construction:

```go
app := service.NewAppWithEnricher(store, clock.RealClock{}, newVocabEnricherFromEnv())
```

Add to `.env.example`:

```text
VOCAB_ENRICHMENT_BASE_URL=https://api.openai.com/v1
VOCAB_ENRICHMENT_API_KEY=
VOCAB_ENRICHMENT_MODEL=gpt-4.1-mini
```

- [ ] **Step 6: Run tests**

```bash
cd backend
gofmt -w cmd internal
go test ./...
```

Expected: all backend tests pass.

- [ ] **Step 7: Commit**

```bash
git add .env.example backend/cmd/api/main.go backend/internal/httpapi backend/internal/service
git commit -m "feat: add vocab autocomplete endpoint"
```

---

### Task 5: Add Web Bulk Import Autocomplete UI

**Files:**
- Modify: `apps/web/src/api.ts`
- Modify: `apps/web/src/App.tsx`
- Modify: `apps/web/src/styles.css`

- [ ] **Step 1: Update API types and client**

In `apps/web/src/api.ts`, add to `VocabItem`:

```ts
part_of_speech: string;
```

Add:

```ts
export interface AutocompleteItem {
  term: string;
  meaning: string;
  example_sentence: string;
  part_of_speech: string;
}

export interface AutocompleteResult extends AutocompleteItem {
  error: string;
}

export async function autocompleteVocab(items: AutocompleteItem[]) {
  return request<{ items: AutocompleteResult[] }>("/vocab/autocomplete", {
    method: "POST",
    body: JSON.stringify({ items })
  });
}
```

- [ ] **Step 2: Update parsed bulk row shape**

In `apps/web/src/App.tsx`, update imports to include `autocompleteVocab`, `AutocompleteItem`, and `AutocompleteResult`.

Change `ParsedImportCard`:

```ts
type ParsedImportCard = {
  term: string;
  meaning: string;
  example_sentence: string;
  part_of_speech: string;
  error?: string;
};
```

Update `parseImportLine` fallback values:

```ts
return {
  term: line.slice(0, index).trim(),
  meaning: line.slice(index + separator.length).trim(),
  example_sentence: "",
  part_of_speech: ""
};
```

Fallback:

```ts
return { term: line.trim(), meaning: "", example_sentence: "", part_of_speech: "" };
```

- [ ] **Step 3: Add enriched bulk state**

After `bulkText` state:

```ts
const [enrichedCards, setEnrichedCards] = useState<ParsedImportCard[]>([]);
const [isEnriching, setIsEnriching] = useState(false);
const [enrichmentError, setEnrichmentError] = useState("");
```

Replace `const parsedImportCards = parseBulkImport(bulkText);` with:

```ts
const parsedImportCards = enrichedCards.length > 0 ? enrichedCards : parseBulkImport(bulkText);
```

When `bulkText` changes, clear enrichment:

```tsx
onChange={(event) => {
  setBulkText(event.target.value);
  setEnrichedCards([]);
  setEnrichmentError("");
}}
```

- [ ] **Step 4: Add fill-empty merge logic**

Add near parser helpers:

```ts
function mergeAutocompleteResults(cards: ParsedImportCard[], results: AutocompleteResult[]): ParsedImportCard[] {
  return cards.map((card, index) => {
    const result = results[index];
    if (!result) return card;
    return {
      ...card,
      meaning: card.meaning || result.meaning || "",
      example_sentence: card.example_sentence || result.example_sentence || "",
      part_of_speech: card.part_of_speech || result.part_of_speech || "",
      error: result.error || undefined
    };
  });
}
```

- [ ] **Step 5: Add handler**

Inside `App`, add:

```ts
async function handleAutocompleteBulk() {
  const cards = parseBulkImport(bulkText);
  if (cards.length === 0) return;
  setIsEnriching(true);
  setEnrichmentError("");
  try {
    const payload: AutocompleteItem[] = cards.map((card) => ({
      term: card.term,
      meaning: card.meaning,
      example_sentence: card.example_sentence,
      part_of_speech: card.part_of_speech
    }));
    const response = await autocompleteVocab(payload);
    setEnrichedCards(mergeAutocompleteResults(cards, response.items));
  } catch (error) {
    setEnrichmentError(error instanceof Error ? error.message : "Autocomplete failed");
  } finally {
    setIsEnriching(false);
  }
}
```

- [ ] **Step 6: Include new fields when importing**

In `handleBulkImport`, update create payload:

```ts
await createVocab({
  term: card.term,
  meaning: card.meaning,
  example_sentence: card.example_sentence,
  part_of_speech: card.part_of_speech,
  kind: "word"
});
```

- [ ] **Step 7: Add UI controls and preview fields**

In the Bulk Import form, add before the Import submit button:

```tsx
<button type="button" className="ghost-button" disabled={isEnriching || parseBulkImport(bulkText).length === 0} onClick={handleAutocompleteBulk}>
  {isEnriching ? "Auto-completing..." : "Auto-complete missing details"}
</button>
{enrichmentError ? <p className="form-error">{enrichmentError}. You can still import manually.</p> : null}
```

In preview card body, add:

```tsx
<span>{card.meaning || "Meaning can be added later."}</span>
{card.example_sentence ? <small>{card.example_sentence}</small> : null}
{card.part_of_speech ? <small className="pos-pill">{card.part_of_speech}</small> : null}
{card.error ? <small className="form-error">{card.error}</small> : null}
```

- [ ] **Step 8: Add small styles**

In `apps/web/src/styles.css`, add:

```css
.pos-pill {
  display: inline-flex;
  width: fit-content;
  border-radius: 999px;
  padding: 0.2rem 0.55rem;
  background: rgba(80, 103, 78, 0.12);
  color: #50674e;
  font-weight: 700;
}

.form-error {
  color: #a33a2b;
  font-weight: 700;
}
```

- [ ] **Step 9: Run web build**

```bash
npm run build:web
```

Expected: Vite build succeeds.

- [ ] **Step 10: Commit**

```bash
git add apps/web/src/api.ts apps/web/src/App.tsx apps/web/src/styles.css
git commit -m "feat: add web bulk vocab autocomplete"
```

---

### Task 6: Final Verification

**Files:**
- No new source files unless previous tasks reveal compile errors.

- [ ] **Step 1: Run backend tests**

```bash
cd backend
go test ./...
```

Expected: all backend packages pass.

- [ ] **Step 2: Run web build**

```bash
npm run build:web
```

Expected: Vite build succeeds.

- [ ] **Step 3: Run integration tests if Postgres is available**

```bash
make test-integration
```

Expected: Postgres repository integration tests pass against `.env.test`.

- [ ] **Step 4: Manual local verification**

Run backend with enrichment env vars:

```bash
VOCAB_ENRICHMENT_BASE_URL=https://api.openai.com/v1 VOCAB_ENRICHMENT_API_KEY=replace-with-real-key VOCAB_ENRICHMENT_MODEL=gpt-4.1-mini make backend-run
```

Run web:

```bash
npm run dev:web
```

In Web Bulk Import, paste:

```text
serendipity
meticulous: very careful
make up - invent or reconcile
```

Click `Auto-complete missing details`.

Expected:

- `serendipity` gets meaning, example, and part of speech.
- `meticulous` keeps `very careful` and gets example plus part of speech.
- `make up` keeps `invent or reconcile` and gets example plus part of speech.
- Import still works after enrichment.

- [ ] **Step 5: Manual failure verification**

Run backend without enrichment env vars:

```bash
make backend-run
```

Click `Auto-complete missing details`.

Expected:

- Web shows a clear error.
- Manual Import button still works.

- [ ] **Step 6: Commit any verification-only fixes**

If previous steps required small fixes:

```bash
git add <changed-files>
git commit -m "fix: stabilize vocab autocomplete"
```

If no fixes were needed, do not create an empty commit.
