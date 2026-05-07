# Vocab Autocomplete Design

## Purpose

Speed up vocabulary creation by generating missing card details from an OpenAI-compatible provider. The first implementation focuses on Web Bulk Import, where the user can paste multiple rows, enrich missing fields on demand, review the results, and then import manually.

## Scope

In scope:

- Web Bulk Import autocomplete only.
- One backend batch request per enrichment action.
- Suggestions for `meaning`, `example_sentence`, and `part_of_speech`.
- Fill empty fields only; never overwrite user-provided row data.
- Store `part_of_speech` on saved vocab cards.
- Keep manual bulk import usable when enrichment is unavailable or fails.

Out of scope:

- Single-card Add Form autocomplete.
- iOS and Chrome extension UI.
- Background enrichment after save.
- Confidence scores or alternative suggestions.
- Notifications work.

## Backend Design

Add an enrichment module under `backend/internal/service/enrichment`. It owns enrichment request validation, provider calls, response parsing, and per-item result normalization. The module depends on a narrow provider interface so unit tests can use fakes.

Add a backend endpoint:

```http
POST /vocab/autocomplete
```

The endpoint requires the same bearer-session authentication as other vocab endpoints.

Request:

```json
{
  "items": [
    {
      "term": "serendipity",
      "meaning": "",
      "example_sentence": "",
      "part_of_speech": ""
    }
  ]
}
```

Response:

```json
{
  "items": [
    {
      "term": "serendipity",
      "meaning": "the occurrence of fortunate events by chance",
      "example_sentence": "Finding that old letter was pure serendipity.",
      "part_of_speech": "noun",
      "error": ""
    }
  ]
}
```

The endpoint only suggests values. It does not save cards.

## Provider Design

Use an OpenAI-compatible chat-completions style adapter configured by environment variables:

```text
VOCAB_ENRICHMENT_BASE_URL
VOCAB_ENRICHMENT_API_KEY
VOCAB_ENRICHMENT_MODEL
```

The backend should still start when these values are missing. In that case, `/vocab/autocomplete` returns a clear error:

```json
{ "error": "vocab enrichment is not configured" }
```

The provider is called once per batch. The first implementation should cap batch size at `20` items.

## Data Model

Add `part_of_speech` to `domain.VocabItem` and the API DTOs.

Database migration:

```sql
part_of_speech TEXT NOT NULL DEFAULT ''
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
)
```

Existing cards migrate with `part_of_speech = ''`. The field remains optional for manual creation.

## Web Bulk Import Flow

Keep the current hand-written parser behavior:

- `term - meaning`
- `term: meaning`
- tab-separated
- term-only lines

Add an `Auto-complete missing details` button in Bulk Import. On click:

1. Parse the current rows.
2. Send all rows to `/vocab/autocomplete`.
3. Fill only missing row fields.
4. Preserve existing meanings or examples.
5. Show enriched preview cards with term, meaning, example sentence, and part of speech.
6. Show per-row errors when a term cannot be enriched.
7. Keep the normal Import button usable even if enrichment fails.

## Error Handling

- Empty batch: `400`.
- More than `20` items: `400`.
- Missing provider config: clear server error response.
- Provider unavailable: clear `502` style response.
- Invalid provider JSON: clear `502` style response.
- Per-item enrichment failure: return item-level `error` with empty suggestion fields for that row; other rows stay usable.

## Testing

Backend:

- Unit tests for enrichment service with a fake provider.
- HTTP handler tests for validation, auth, and error mapping.
- Provider adapter tests for request shape and strict JSON parsing.
- Migration/repository tests proving `part_of_speech` persists and invalid values are rejected.

Web:

- Manual verification for term-only rows.
- Manual verification for mixed rows where existing meaning is preserved.
- Manual verification that failed enrichment does not block manual import.

## Implementation Order

1. Add `part_of_speech` migration and model plumbing.
2. Add enrichment service and fake-provider tests.
3. Add OpenAI-compatible provider adapter.
4. Add `/vocab/autocomplete` endpoint.
5. Add Web Bulk Import API client and UI state.
6. Verify backend tests and web build.
