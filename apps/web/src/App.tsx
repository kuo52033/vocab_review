import { FormEvent, useEffect, useRef, useState } from "react";
import {
  autocompleteVocab,
  AutocompleteItem,
  AutocompleteResult,
  createVocab,
  deleteVocab,
  getReviewStats,
  gradeReview,
  listDue,
  listNotificationJobs,
  listVocab,
  requestMagicLink,
  ReviewGrade,
  ReviewStats,
  setToken,
  updateVocab,
  verifyMagicLink,
  VocabItem,
  VocabWithState
} from "./api";

type CardDraft = {
  term: string;
  meaning: string;
  example_sentence: string;
  notes: string;
};

type AuthState = {
  email: string;
  magicLink?: string;
  token: string;
};

type ActiveSection = "review" | "add" | "library";
type SessionSummary = {
  reviewed: number;
  correct: number;
  wrong: number;
  lastNextDue?: string;
};
type QuizOption = {
  id: string;
  text: string;
  isCorrect: boolean;
};
type QuizCard = {
  card: VocabWithState;
  options: QuizOption[];
};
type ParsedImportCard = {
  term: string;
  meaning: string;
  example_sentence: string;
  part_of_speech: string;
  error?: string;
};

const allowedPartsOfSpeech = new Set([
  "",
  "noun",
  "verb",
  "adjective",
  "adverb",
  "phrase",
  "idiom",
  "phrasal_verb",
  "preposition",
  "conjunction",
  "interjection",
  "determiner",
  "pronoun",
  "other"
]);

const emptyForm: CardDraft = {
  term: "",
  meaning: "",
  example_sentence: "",
  notes: ""
};

const emptyStats: ReviewStats = {
  reviewed_today: 0,
  reviewed_7_days: 0,
  active_cards: 0,
  due_now: 0,
  archived_cards: 0
};

const reviewSessionSize = 12;
const libraryPageSize = 10;

function draftFromItem(item: VocabItem): CardDraft {
  return {
    term: item.term,
    meaning: item.meaning,
    example_sentence: item.example_sentence,
    notes: item.notes
  };
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value));
}

function parseImportLine(line: string): ParsedImportCard {
  if (line.includes("|")) {
    const [term = "", meaning = "", example_sentence = "", part_of_speech = ""] = line.split("|").map((part) => part.trim());
    return { term, meaning, example_sentence, part_of_speech };
  }
  const separators = [" - ", "\t", ": ", "："];
  for (const separator of separators) {
    const index = line.indexOf(separator);
    if (index === -1) continue;
    return {
      term: line.slice(0, index).trim(),
      meaning: line.slice(index + separator.length).trim(),
      example_sentence: "",
      part_of_speech: ""
    };
  }
  return { term: line.trim(), meaning: "", example_sentence: "", part_of_speech: "" };
}

function parseBulkImport(input: string) {
  return input
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map(parseImportLine)
    .filter((card) => card.term);
}

function pageCount(totalItems: number, pageSize: number) {
  return Math.max(1, Math.ceil(totalItems / pageSize));
}

function pageItems<T>(items: T[], page: number, pageSize: number) {
  const start = (page - 1) * pageSize;
  return items.slice(start, start + pageSize);
}

function shuffleItems<T>(items: T[]) {
  return [...items].sort(() => Math.random() - 0.5);
}

function answerText(item: VocabItem) {
  return item.meaning.trim();
}

function buildQuizDeck(dueCards: VocabWithState[], candidates: VocabWithState[], limit: number): QuizCard[] {
  const cardsWithAnswers = dueCards.filter(({ item }) => answerText(item));
  const candidateAnswers = candidates
    .filter(({ item }) => answerText(item))
    .map(({ item }) => ({ id: item.id, text: answerText(item) }));

  return shuffleItems(cardsWithAnswers)
    .slice(0, limit)
    .map((card) => {
      const correctText = answerText(card.item);
      const distractors = shuffleItems(
        candidateAnswers.filter((candidate) => candidate.id !== card.item.id && candidate.text !== correctText)
      ).slice(0, 3);

      return {
        card,
        options: shuffleItems([
          { id: `${card.item.id}-correct`, text: correctText, isCorrect: true },
          ...distractors.map((distractor) => ({
            id: `${card.item.id}-${distractor.id}`,
            text: distractor.text,
            isCorrect: false
          }))
        ])
      };
    })
    .filter((quizCard) => quizCard.options.length >= 2);
}

type PaginationProps = {
  label: string;
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
};

function Pagination({ label, page, totalPages, onPageChange }: PaginationProps) {
  if (totalPages <= 1) return null;

  return (
    <nav className="pagination" aria-label={`${label} pagination`}>
      <button type="button" className="ghost-button compact-button" onClick={() => onPageChange(page - 1)} disabled={page <= 1}>
        Previous
      </button>
      <span>
        Page {page} of {totalPages}
      </span>
      <button type="button" className="ghost-button compact-button" onClick={() => onPageChange(page + 1)} disabled={page >= totalPages}>
        Next
      </button>
    </nav>
  );
}

function normalizePartOfSpeech(value: string) {
  const normalized = value.trim().toLowerCase().replace(/[\s-]+/g, "_");
  if (allowedPartsOfSpeech.has(normalized)) return normalized;
  return normalized ? "other" : "";
}

function mergeAutocompleteResults(cards: ParsedImportCard[], results: AutocompleteResult[]): ParsedImportCard[] {
  return cards.map((card, index) => {
    const result = results[index];
    if (!result) return card;
    const partOfSpeech = normalizePartOfSpeech(result.part_of_speech || "");
    return {
      ...card,
      meaning: card.meaning || result.meaning || "",
      example_sentence: card.example_sentence || result.example_sentence || "",
      part_of_speech: card.part_of_speech || partOfSpeech,
      error: result.error || undefined
    };
  });
}

function formatBulkImportCards(cards: ParsedImportCard[]) {
  return cards
    .map((card) => {
      const fields = [
        card.term,
        card.meaning,
        card.example_sentence,
        card.part_of_speech
      ].map((value) => value.trim());
      while (fields.length > 1 && fields[fields.length - 1] === "") {
        fields.pop();
      }
      return fields.join(" | ");
    })
    .join("\n");
}

export function App() {
  const termInputRef = useRef<HTMLInputElement>(null);
  const bulkTextRef = useRef("");
  const [auth, setAuth] = useState<AuthState>({ email: "", token: localStorage.getItem("session_token") ?? "" });
  const [vocab, setVocab] = useState<VocabWithState[]>([]);
  const [due, setDue] = useState<VocabWithState[]>([]);
  const [vocabTotal, setVocabTotal] = useState(0);
  const [stats, setStats] = useState<ReviewStats>(emptyStats);
  const [jobs, setJobs] = useState<Array<{ id: string; vocab_item_id: string; status: string; scheduled_at: string }>>([]);
  const [form, setForm] = useState(emptyForm);
  const [bulkText, setBulkText] = useState("");
  const [enrichedCards, setEnrichedCards] = useState<ParsedImportCard[] | null>(null);
  const [isEnriching, setIsEnriching] = useState(false);
  const [enrichmentError, setEnrichmentError] = useState("");
  const [editingID, setEditingID] = useState("");
  const [editDraft, setEditDraft] = useState<CardDraft>(emptyForm);
  const [query, setQuery] = useState("");
  const [activeSection, setActiveSection] = useState<ActiveSection>("review");
  const [libraryPage, setLibraryPage] = useState(1);
  const [sessionDeck, setSessionDeck] = useState<QuizCard[]>([]);
  const [sessionIndex, setSessionIndex] = useState(0);
  const [sessionCorrectCount, setSessionCorrectCount] = useState(0);
  const [sessionWrongCount, setSessionWrongCount] = useState(0);
  const [sessionSummary, setSessionSummary] = useState<SessionSummary | null>(null);
  const [selectedOptionID, setSelectedOptionID] = useState("");
  const [pendingNextDue, setPendingNextDue] = useState("");
  const [lastCreatedTerm, setLastCreatedTerm] = useState("");
  const [lastImportCount, setLastImportCount] = useState(0);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isStartingReview, setIsStartingReview] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isGrading, setIsGrading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const token = new URLSearchParams(window.location.search).get("token");
    if (token) {
      verifyMagicLink(token)
        .then((result) => {
          setToken(result.session.token);
          setAuth((current) => ({ ...current, token: result.session.token }));
          window.history.replaceState({}, "", window.location.pathname);
        })
        .catch((err: Error) => setError(err.message));
    }
  }, []);

  useEffect(() => {
    if (!auth.token) return;
    refresh();
  }, [auth.token, libraryPage, query]);

  const normalizedQuery = query.trim().toLowerCase();
  const visibleVocab = [...vocab]
    .sort((left, right) => new Date(right.item.created_at).getTime() - new Date(left.item.created_at).getTime());

  useEffect(() => {
    setLibraryPage((current) => Math.min(current, pageCount(vocabTotal, libraryPageSize)));
  }, [vocabTotal]);

  const currentQuizCard = sessionDeck[sessionIndex];
  const sessionActive = Boolean(currentQuizCard);
  const sessionProgress = sessionDeck.length > 0 ? Math.round((sessionIndex / sessionDeck.length) * 100) : 0;
  const libraryPageCount = pageCount(vocabTotal, libraryPageSize);
  const paginatedVocab = visibleVocab;
  const rawImportCards = parseBulkImport(bulkText);
  const parsedImportCards = enrichedCards ?? rawImportCards;

  async function refresh() {
    setIsRefreshing(true);
    try {
      const [vocabResponse, dueResponse, statsResponse, jobsResponse] = await Promise.all([
        listVocab({
          limit: libraryPageSize,
          offset: (libraryPage - 1) * libraryPageSize,
          q: normalizedQuery
        }),
        listDue(),
        getReviewStats(),
        listNotificationJobs()
      ]);
      setVocab(vocabResponse.items);
      setDue(dueResponse.items);
      setVocabTotal(vocabResponse.total);
      setStats(statsResponse.stats);
      setJobs(jobsResponse.items);
      setError("");
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setIsRefreshing(false);
    }
  }

  async function handleRequestLink(event: FormEvent) {
    event.preventDefault();
    try {
      const response = await requestMagicLink(auth.email);
      setAuth((current) => ({ ...current, magicLink: response.verification_url }));
      setError("");
    } catch (err) {
      setError((err as Error).message);
    }
  }

  function applyCreatedCard(createdCard: VocabWithState) {
    const isDueNow = new Date(createdCard.state.next_due_at).getTime() <= Date.now();
    setVocab((current) => [createdCard, ...current].slice(0, libraryPageSize));
    setVocabTotal((current) => current + 1);
    if (isDueNow) {
      setDue((current) => [createdCard, ...current]);
    }
    setStats((current) => ({
      ...current,
      active_cards: current.active_cards + 1,
      due_now: current.due_now + (isDueNow ? 1 : 0)
    }));
  }

  async function handleCreateVocab(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSaving(true);
    try {
      const response = await createVocab(form);
      const createdCard = { item: response.item, state: response.state };
      setForm(emptyForm);
      applyCreatedCard(createdCard);
      setLastCreatedTerm(response.item.term);
      setLastImportCount(0);
      setError("");
      requestAnimationFrame(() => termInputRef.current?.focus());
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setIsSaving(false);
    }
  }

  async function handleBulkImport(event: FormEvent) {
    event.preventDefault();
    if (parsedImportCards.length === 0) return;

    setIsSaving(true);
    let importedCount = 0;
    try {
      for (const card of parsedImportCards) {
        const response = await createVocab({
          term: card.term,
          meaning: card.meaning,
          example_sentence: card.example_sentence,
          part_of_speech: card.part_of_speech,
          notes: ""
        });
        applyCreatedCard({ item: response.item, state: response.state });
        importedCount += 1;
      }
      handleBulkTextChange("");
      setEnrichedCards(null);
      setEnrichmentError("");
      setLastCreatedTerm("");
      setLastImportCount(importedCount);
      setError("");
    } catch (err) {
      setLastImportCount(importedCount);
      setError((err as Error).message);
    } finally {
      setIsSaving(false);
    }
  }

  async function handleAutocompleteBulk() {
    const inputSnapshot = bulkText;
    const cards = parseBulkImport(inputSnapshot);
    if (cards.length === 0) return;

    setIsEnriching(true);
    setEnrichmentError("");
    try {
      const items: AutocompleteItem[] = cards.map(({ term, meaning, example_sentence, part_of_speech }) => ({
        term,
        meaning,
        example_sentence,
        part_of_speech
      }));
      const response = await autocompleteVocab(items);
      if (bulkTextRef.current !== inputSnapshot) return;
      const mergedCards = mergeAutocompleteResults(cards, response.items);
      const mergedText = formatBulkImportCards(mergedCards);
      bulkTextRef.current = mergedText;
      setBulkText(mergedText);
      setEnrichedCards(mergedCards);
    } catch (err) {
      if (bulkTextRef.current !== inputSnapshot) return;
      setEnrichmentError(`${(err as Error).message}. Manual import still works with the details currently shown.`);
    } finally {
      setIsEnriching(false);
    }
  }

  async function handleSaveEdit(event: FormEvent) {
    event.preventDefault();
    if (!editingID) return;
    setIsSaving(true);
    try {
      await updateVocab(editingID, editDraft);
      setEditingID("");
      setEditDraft(emptyForm);
      await refresh();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setIsSaving(false);
    }
  }

  async function handleArchive(id: string) {
    setIsSaving(true);
    try {
      await deleteVocab(id);
      if (editingID === id) {
        setEditingID("");
        setEditDraft(emptyForm);
      }
      await refresh();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setIsSaving(false);
    }
  }

  async function handleGrade(id: string, grade: "again" | "hard" | "good" | "easy") {
    setIsGrading(true);
    try {
      await gradeReview(id, grade);
      await refresh();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setIsGrading(false);
    }
  }

  async function startReviewSession() {
    if (due.length === 0) return;
    setIsStartingReview(true);
    try {
      const vocabResponse = await listVocab({ limit: 100, offset: 0 });
      const deck = buildQuizDeck(due, vocabResponse.items, reviewSessionSize);
      if (deck.length === 0) {
        setError("Start Review needs at least one due card with a meaning and one other active card with a meaning.");
        return;
      }
      setSessionDeck(deck);
      setSessionIndex(0);
      setSessionCorrectCount(0);
      setSessionWrongCount(0);
      setSessionSummary(null);
      setSelectedOptionID("");
      setError("");
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setIsStartingReview(false);
    }
  }

  function endReviewSession() {
    setSessionDeck([]);
    setSessionIndex(0);
    setSessionCorrectCount(0);
    setSessionWrongCount(0);
    setSelectedOptionID("");
    setPendingNextDue("");
  }

  async function handleQuizAnswer(option: QuizOption) {
    if (!currentQuizCard || selectedOptionID) return;
    setSelectedOptionID(option.id);
    setIsGrading(true);
    try {
      const grade: ReviewGrade = option.isCorrect ? "easy" : "again";
      const response = await gradeReview(currentQuizCard.card.item.id, grade);
      const reviewed = sessionIndex + 1;
      const correct = sessionCorrectCount + (option.isCorrect ? 1 : 0);
      const wrong = sessionWrongCount + (option.isCorrect ? 0 : 1);
      setSessionCorrectCount(correct);
      setSessionWrongCount(wrong);
      await refresh();
      setPendingNextDue(response.state.next_due_at);
      setError("");
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setIsGrading(false);
    }
  }

  function advanceQuizCard() {
    const reviewed = sessionIndex + 1;
    if (reviewed >= sessionDeck.length) {
      setSessionSummary({
        reviewed,
        correct: sessionCorrectCount,
        wrong: sessionWrongCount,
        lastNextDue: pendingNextDue
      });
      endReviewSession();
      return;
    }
    setSessionIndex(reviewed);
    setSelectedOptionID("");
    setPendingNextDue("");
  }

  function startEditing(item: VocabItem) {
    setEditingID(item.id);
    setEditDraft(draftFromItem(item));
  }

  function handleBulkTextChange(value: string) {
    bulkTextRef.current = value;
    setBulkText(value);
    setEnrichedCards(null);
    setEnrichmentError("");
  }

  if (!auth.token) {
    return (
      <main className="shell auth-shell">
        <section className="panel auth">
          <p className="eyebrow">Vocabulary review system</p>
          <h1>Sign in with a magic link</h1>
          <form onSubmit={handleRequestLink} className="stack">
            <input
              type="email"
              placeholder="you@example.com"
              value={auth.email}
              onChange={(event) => setAuth((current) => ({ ...current, email: event.target.value }))}
            />
            <button type="submit">Create login link</button>
          </form>
          {auth.magicLink ? (
            <div className="callout">
              <p>Development magic link</p>
              <a href={auth.magicLink}>{auth.magicLink}</a>
            </div>
          ) : null}
          {error ? <p className="error">{error}</p> : null}
        </section>
      </main>
    );
  }

  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-heading">
          <p className="eyebrow">Vocab Review</p>
          <div className="sidebar-utilities">
            <button type="button" className="icon-button sidebar-add" onClick={() => setActiveSection("add")} aria-label="Add card">
              +
            </button>
            <button type="button" className="icon-button sidebar-refresh" onClick={refresh} disabled={isRefreshing} aria-label="Refresh data">
              ↻
            </button>
          </div>
        </div>

        <nav className="sidebar-nav" aria-label="Workspace sections">
          {[
            ["review", "Start Review", `${stats.due_now} due`],
            ["library", "Active cards", `${stats.active_cards} cards`]
          ].map(([section, label, detail]) => (
            <button
              key={section}
              type="button"
              className={activeSection === section ? "nav-item active" : "nav-item"}
              onClick={() => setActiveSection(section as ActiveSection)}
            >
              <span>{label}</span>
              <small>{detail}</small>
            </button>
          ))}
        </nav>

        <div className="sidebar-stats">
          <article>
            <strong>{stats.due_now}</strong>
            <span>Due now</span>
          </article>
          <article>
            <strong>{stats.reviewed_today}</strong>
            <span>Reviewed today</span>
          </article>
          <article>
            <strong>{stats.active_cards}</strong>
            <span>Active cards</span>
          </article>
          <article>
            <strong>{stats.reviewed_7_days}</strong>
            <span>7 day reviews</span>
          </article>
        </div>
      </aside>

      <section className="workspace">
        {activeSection === "review" ? (
          <section className="panel due-panel">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Today</p>
                <h2>Start Review</h2>
                <small>Answer one card at a time. Each session uses up to {reviewSessionSize} due words.</small>
              </div>
            </div>
            {sessionActive ? (
              <article className="session-card">
                <div className="session-topline">
                  <span>
                    Card {sessionIndex + 1} of {sessionDeck.length}
                  </span>
                  <button type="button" className="ghost-button compact-button" onClick={endReviewSession} disabled={isGrading}>
                    End
                  </button>
                </div>
                <div className="session-progress" aria-label={`${sessionProgress}% complete`}>
                  <span style={{ width: `${sessionProgress}%` }} />
                </div>
                <div className="session-prompt">
                  <p className="eyebrow">Word</p>
                  <h3>{currentQuizCard.card.item.term}</h3>
                  <small>Choose the correct meaning.</small>
                </div>
                <div className="answer-options" aria-label="Meaning choices">
                  {currentQuizCard.options.map((option, index) => {
                    const isSelected = selectedOptionID === option.id;
                    const showCorrect = Boolean(selectedOptionID) && option.isCorrect;
                    const showWrong = isSelected && !option.isCorrect;
                    return (
                      <button
                        key={option.id}
                        type="button"
                        className={[
                          "answer-option",
                          isSelected ? "selected" : "",
                          showCorrect ? "correct" : "",
                          showWrong ? "wrong" : ""
                        ].filter(Boolean).join(" ")}
                        onClick={() => handleQuizAnswer(option)}
                        disabled={Boolean(selectedOptionID) || isGrading}
                      >
                        <span>{String.fromCharCode(65 + index)}</span>
                        <strong>{option.text}</strong>
                      </button>
                    );
                  })}
                </div>
                <div className="quiz-feedback" aria-live="polite">
                  {selectedOptionID ? (
                    currentQuizCard.options.find((option) => option.id === selectedOptionID)?.isCorrect ? (
                      <span className="correct-text">Correct. This card will move further out.</span>
                    ) : (
                      <span className="wrong-text">Not this one. The card will return sooner.</span>
                    )
                  ) : (
                    <span>Correct answers use the existing easy grade. Wrong answers use again.</span>
                  )}
                </div>
                {selectedOptionID ? (
                  <button type="button" className="next-card-button" onClick={advanceQuizCard} disabled={isGrading}>
                    {sessionIndex + 1 >= sessionDeck.length ? "Show summary" : "Next word"}
                  </button>
                ) : null}
              </article>
            ) : (
              <div className="review-start-card">
                <span className="due-pill">{stats.due_now} due now</span>
                <div className="review-card-copy">
                  <p className="eyebrow">Quiz mode</p>
                  <h3>{stats.due_now === 0 ? "Clear desk." : "Ready when you are."}</h3>
                  <span>{stats.due_now === 0 ? "No due cards right now." : "A short multiple-choice sprint is waiting."}</span>
                </div>
                <div className="review-card-action">
                  <button type="button" onClick={startReviewSession} disabled={due.length === 0 || isStartingReview}>
                    {stats.due_now === 0 ? "All caught up" : isStartingReview ? "Preparing..." : "Start Review"}
                  </button>
                </div>
              </div>
            )}
            {sessionSummary ? (
              <div className="session-summary quiz-summary">
                <div>
                  <strong>Review complete.</strong>
                  <span>
                    {sessionSummary.correct} correct, {sessionSummary.wrong} wrong.
                  </span>
                </div>
                <div>
                  <strong>{Math.round((sessionSummary.correct / Math.max(1, sessionSummary.reviewed)) * 100)}%</strong>
                  <span>accuracy</span>
                </div>
                {sessionSummary.lastNextDue ? <span>Last card returns {formatDate(sessionSummary.lastNextDue)}.</span> : null}
              </div>
            ) : null}
          </section>
        ) : null}

        {activeSection === "add" ? (
          <section className="panel quick-add">
            <form className="capture-card stack" onSubmit={handleCreateVocab}>
              <div className="section-heading">
                <div>
                  <p className="eyebrow">Capture</p>
                  <h2>Quick add</h2>
                  <small>Add one card fast. Only the word is required.</small>
                </div>
              </div>
              <label className="field-label">
                <span>Word</span>
                <div className="quick-capture-line">
                  <input
                    ref={termInputRef}
                    value={form.term}
                    placeholder="e.g. meticulous"
                    onChange={(event) => setForm({ ...form, term: event.target.value })}
                  />
                </div>
              </label>
              <div className="optional-grid">
                <label className="field-label">
                  <span>Meaning</span>
                  <textarea value={form.meaning} placeholder="Short definition" onChange={(event) => setForm({ ...form, meaning: event.target.value })} />
                </label>
                <label className="field-label">
                  <span>Example sentence</span>
                  <textarea
                    value={form.example_sentence}
                    placeholder="Use it in context"
                    onChange={(event) => setForm({ ...form, example_sentence: event.target.value })}
                  />
                </label>
                <label className="field-label">
                  <span>Notes</span>
                  <textarea value={form.notes} placeholder="Memory hint or source" onChange={(event) => setForm({ ...form, notes: event.target.value })} />
                </label>
              </div>
              <div className="action-row quick-actions">
                <button type="submit" disabled={isSaving || !form.term.trim()}>
                  {isSaving ? "Saving..." : "Save"}
                </button>
              </div>
              {lastCreatedTerm ? <p className="save-confirmation">Saved "{lastCreatedTerm}". Ready for the next one.</p> : null}
            </form>

            <form className="capture-card bulk-import" onSubmit={handleBulkImport}>
              <div className="section-heading import-heading">
                <div>
                  <p className="eyebrow">Batch capture</p>
                  <h2>Bulk import</h2>
                </div>
                <button
                  type="button"
                  className="ghost-button compact-button ai-button"
                  disabled={isEnriching || rawImportCards.length === 0}
                  onClick={handleAutocompleteBulk}
                >
                  <span className="gpt-mark" aria-hidden="true">✦</span>
                  {isEnriching ? "Working..." : "GPT Auto-complete"}
                </button>
              </div>
              <textarea
                className="bulk-textarea"
                value={bulkText}
                placeholder={"abandon - to leave behind\nmeticulous: very careful\nmake up"}
                onChange={(event) => handleBulkTextChange(event.target.value)}
              />
              {parsedImportCards.length > 0 ? (
                <div className="import-preview">
                  {parsedImportCards.slice(0, 8).map((card, index) => (
                    <article key={`${card.term}-${index}`} className="import-preview-card">
                      <strong>{card.term}</strong>
                      {card.part_of_speech ? <span className="pos-pill">{card.part_of_speech}</span> : null}
                      <span>{card.meaning || "Meaning can be added later."}</span>
                      {card.example_sentence ? <span>{card.example_sentence}</span> : null}
                      {card.error ? <span className="form-error">{card.error}</span> : null}
                    </article>
                  ))}
                  {parsedImportCards.length > 8 ? <span className="muted">+ {parsedImportCards.length - 8} more</span> : null}
                </div>
              ) : null}
              {enrichmentError ? <p className="form-error">{enrichmentError}</p> : null}
              <div className="action-row bulk-actions">
                <button type="submit" disabled={isSaving || parsedImportCards.length === 0}>
                  {isSaving ? "Saving..." : "Save"}
                </button>
              </div>
              {lastImportCount ? (
                <p className="save-confirmation">
                  Imported {lastImportCount} card{lastImportCount === 1 ? "" : "s"}.
                </p>
              ) : null}
            </form>
          </section>
        ) : null}

        {activeSection === "library" ? (
          <section className="panel library-panel">
            <div className="library-toolbar">
              <div>
                <p className="eyebrow">Library</p>
                <h2>Manage active cards</h2>
              </div>
            </div>

            <div className="filters">
              <input
                className="search-input"
                value={query}
                placeholder="Search term, meaning, example, notes..."
                onChange={(event) => {
                  setQuery(event.target.value);
                  setLibraryPage(1);
                }}
              />
            </div>

            <div className="library">
              {paginatedVocab.map(({ item, state }) => (
                <article className={editingID === item.id ? "library-card editing" : "library-card"} key={item.id}>
                  {editingID === item.id ? (
                    <form className="edit-form" onSubmit={handleSaveEdit}>
                      <label className="field-label">
                        <span>Word</span>
                        <input value={editDraft.term} onChange={(event) => setEditDraft({ ...editDraft, term: event.target.value })} />
                      </label>
                      <label className="field-label">
                        <span>Meaning</span>
                        <textarea value={editDraft.meaning} onChange={(event) => setEditDraft({ ...editDraft, meaning: event.target.value })} />
                      </label>
                      <label className="field-label">
                        <span>Example sentence</span>
                        <textarea
                          value={editDraft.example_sentence}
                          onChange={(event) => setEditDraft({ ...editDraft, example_sentence: event.target.value })}
                        />
                      </label>
                      <label className="field-label">
                        <span>Notes</span>
                        <textarea value={editDraft.notes} onChange={(event) => setEditDraft({ ...editDraft, notes: event.target.value })} />
                      </label>
                      <div className="action-row">
                        <button type="submit" disabled={isSaving}>{isSaving ? "Saving..." : "Save changes"}</button>
                        <button type="button" className="ghost-button" onClick={() => setEditingID("")}>Cancel</button>
                      </div>
                    </form>
                  ) : (
                    <details className="library-disclosure">
                      <summary>
                        <span className="library-word">{item.term}</span>
                        <span className="library-actions">
                          <button type="button" className="icon-button ghost-button" aria-label={`Edit ${item.term}`} onClick={(event) => {
                            event.preventDefault();
                            startEditing(item);
                          }}>
                            ✎
                          </button>
                          <button type="button" className="icon-button danger-button" aria-label={`Archive ${item.term}`} disabled={isSaving} onClick={(event) => {
                            event.preventDefault();
                            handleArchive(item.id);
                          }}>
                            ×
                          </button>
                        </span>
                      </summary>
                      <div className="library-details">
                        <span className="part-of-speech-badge">{item.part_of_speech || "part of speech not set"}</span>
                        <p>{item.meaning || "Meaning not added yet."}</p>
                        {item.example_sentence ? <small>{item.example_sentence}</small> : null}
                        {item.notes ? <small className="notes">Notes: {item.notes}</small> : null}
                      </div>
                    </details>
                  )}
                </article>
              ))}
            </div>
            <Pagination label="Active cards" page={libraryPage} totalPages={libraryPageCount} onPageChange={setLibraryPage} />

            {vocabTotal === 0 ? (
              <div className="empty-state spacious">
                <strong>{stats.active_cards === 0 ? "No cards yet." : "No matching cards."}</strong>
                <span>{stats.active_cards === 0 ? "Add your first word above." : "Try another search."}</span>
              </div>
            ) : null}
          </section>
        ) : null}

        {error ? <p className="error floating-error">{error}</p> : null}
      </section>
    </main>
  );
}
