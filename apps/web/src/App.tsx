import { FormEvent, useEffect, useRef, useState } from "react";
import {
  createVocab,
  deleteVocab,
  getReviewStats,
  gradeReview,
  listDue,
  listNotificationJobs,
  listReviewHistory,
  listVocab,
  requestMagicLink,
  ReviewGrade,
  ReviewHistoryEntry,
  ReviewStats,
  setToken,
  updateVocab,
  verifyMagicLink,
  VocabItem,
  VocabWithState
} from "./api";

type CardDraft = {
  term: string;
  kind: "word" | "phrase";
  meaning: string;
  example_sentence: string;
  notes: string;
};

type AuthState = {
  email: string;
  magicLink?: string;
  token: string;
};

type StatusFilter = "all" | "new" | "learning" | "review";
type ActiveSection = "review" | "add" | "history" | "library";
type SessionSummary = {
  reviewed: number;
  again: number;
  lastNextDue?: string;
};
type ParsedImportCard = {
  term: string;
  meaning: string;
};

const emptyForm: CardDraft = {
  term: "",
  kind: "word",
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

function draftFromItem(item: VocabItem): CardDraft {
  return {
    term: item.term,
    kind: item.kind,
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
  const separators = [" - ", "\t", ": ", "："];
  for (const separator of separators) {
    const index = line.indexOf(separator);
    if (index === -1) continue;
    return {
      term: line.slice(0, index).trim(),
      meaning: line.slice(index + separator.length).trim()
    };
  }
  return { term: line.trim(), meaning: "" };
}

function parseBulkImport(input: string) {
  return input
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map(parseImportLine)
    .filter((card) => card.term);
}

export function App() {
  const termInputRef = useRef<HTMLInputElement>(null);
  const [auth, setAuth] = useState<AuthState>({ email: "", token: localStorage.getItem("session_token") ?? "" });
  const [vocab, setVocab] = useState<VocabWithState[]>([]);
  const [due, setDue] = useState<VocabWithState[]>([]);
  const [history, setHistory] = useState<ReviewHistoryEntry[]>([]);
  const [stats, setStats] = useState<ReviewStats>(emptyStats);
  const [jobs, setJobs] = useState<Array<{ id: string; vocab_item_id: string; status: string; scheduled_at: string }>>([]);
  const [form, setForm] = useState(emptyForm);
  const [bulkText, setBulkText] = useState("");
  const [editingID, setEditingID] = useState("");
  const [editDraft, setEditDraft] = useState<CardDraft>(emptyForm);
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [activeSection, setActiveSection] = useState<ActiveSection>("review");
  const [selectedHistory, setSelectedHistory] = useState<ReviewHistoryEntry | null>(null);
  const [sessionDeck, setSessionDeck] = useState<VocabWithState[]>([]);
  const [sessionIndex, setSessionIndex] = useState(0);
  const [sessionAgainCount, setSessionAgainCount] = useState(0);
  const [sessionSummary, setSessionSummary] = useState<SessionSummary | null>(null);
  const [isAnswerRevealed, setIsAnswerRevealed] = useState(false);
  const [lastCreatedTerm, setLastCreatedTerm] = useState("");
  const [lastImportCount, setLastImportCount] = useState(0);
  const [isRefreshing, setIsRefreshing] = useState(false);
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
  }, [auth.token]);

  const normalizedQuery = query.trim().toLowerCase();
  const visibleVocab = vocab
    .filter(({ item }) => !item.archived_at)
    .filter(({ state }) => statusFilter === "all" || state.status === statusFilter)
    .filter(({ item }) => {
      if (!normalizedQuery) return true;
      return [item.term, item.meaning, item.example_sentence, item.notes].join(" ").toLowerCase().includes(normalizedQuery);
    })
    .sort((left, right) => new Date(right.item.created_at).getTime() - new Date(left.item.created_at).getTime());
  const currentSessionCard = sessionDeck[sessionIndex];
  const sessionActive = Boolean(currentSessionCard);
  const sessionProgress = sessionDeck.length > 0 ? Math.round((sessionIndex / sessionDeck.length) * 100) : 0;
  const parsedImportCards = parseBulkImport(bulkText);

  async function refresh() {
    setIsRefreshing(true);
    try {
      const [vocabResponse, dueResponse, historyResponse, statsResponse, jobsResponse] = await Promise.all([
        listVocab(),
        listDue(),
        listReviewHistory(),
        getReviewStats(),
        listNotificationJobs()
      ]);
      setVocab(vocabResponse.items);
      setDue(dueResponse.items);
      setHistory(historyResponse.items);
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
    setVocab((current) => [createdCard, ...current]);
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
    const submitter = (event.nativeEvent as SubmitEvent).submitter as HTMLButtonElement | null;
    const nextAction = submitter?.name === "save-review" ? "review" : "add";
    setIsSaving(true);
    try {
      const response = await createVocab(form);
      const createdCard = { item: response.item, state: response.state };
      setForm(emptyForm);
      applyCreatedCard(createdCard);
      setLastCreatedTerm(response.item.term);
      setLastImportCount(0);
      setError("");
      if (nextAction === "review") {
        setActiveSection("review");
      } else {
        requestAnimationFrame(() => termInputRef.current?.focus());
      }
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
          kind: "word",
          meaning: card.meaning,
          example_sentence: "",
          notes: ""
        });
        applyCreatedCard({ item: response.item, state: response.state });
        importedCount += 1;
      }
      setBulkText("");
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

  function startReviewSession() {
    if (due.length === 0) return;
    setSessionDeck(due);
    setSessionIndex(0);
    setSessionAgainCount(0);
    setSessionSummary(null);
    setIsAnswerRevealed(false);
    setError("");
  }

  function endReviewSession() {
    setSessionDeck([]);
    setSessionIndex(0);
    setSessionAgainCount(0);
    setIsAnswerRevealed(false);
  }

  async function handleSessionGrade(grade: ReviewGrade) {
    if (!currentSessionCard) return;
    setIsGrading(true);
    try {
      const response = await gradeReview(currentSessionCard.item.id, grade);
      const reviewed = sessionIndex + 1;
      const again = sessionAgainCount + (grade === "again" ? 1 : 0);
      setSessionAgainCount(again);
      await refresh();
      if (reviewed >= sessionDeck.length) {
        setSessionSummary({ reviewed, again, lastNextDue: response.state.next_due_at });
        endReviewSession();
      } else {
        setSessionIndex(reviewed);
        setIsAnswerRevealed(false);
      }
      setError("");
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setIsGrading(false);
    }
  }

  function startEditing(item: VocabItem) {
    setEditingID(item.id);
    setEditDraft(draftFromItem(item));
  }

  function toggleHistoryDetail(entry: ReviewHistoryEntry) {
    setSelectedHistory((current) => (current?.log.id === entry.log.id ? null : entry));
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
        <div>
          <p className="eyebrow">Vocab Review</p>
          <h1>A quiet desk for keeping words alive.</h1>
        </div>

        <nav className="sidebar-nav" aria-label="Workspace sections">
          {[
            ["review", "Due review", `${due.length} due`],
            ["add", "Add card", "Capture"],
            ["history", "Recent reviews", `${history.length} logs`],
            ["library", "Active cards", `${visibleVocab.length} cards`]
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
          <article>
            <strong>{stats.archived_cards}</strong>
            <span>Archived</span>
          </article>
        </div>

        <button type="button" className="ghost-button" onClick={refresh} disabled={isRefreshing}>
          {isRefreshing ? "Refreshing..." : "Refresh data"}
        </button>
      </aside>

      <section className="workspace">
        {activeSection === "review" ? (
          <section className="panel due-panel">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Today</p>
                <h2>Due now</h2>
              </div>
              <button type="button" className="compact-button" onClick={startReviewSession} disabled={due.length === 0 || sessionActive}>
                {sessionActive ? "Session running" : "Start session"}
              </button>
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
                  <p className="eyebrow">{currentSessionCard.item.kind}</p>
                  <h3>{currentSessionCard.item.term}</h3>
                  {currentSessionCard.item.example_sentence ? <small>{currentSessionCard.item.example_sentence}</small> : null}
                </div>
                {isAnswerRevealed ? (
                  <div className="answer-panel">
                    <strong>Answer</strong>
                    <p>{currentSessionCard.item.meaning || "Meaning not added yet."}</p>
                    {currentSessionCard.item.notes ? <small className="notes">Notes: {currentSessionCard.item.notes}</small> : null}
                  </div>
                ) : (
                  <button type="button" className="reveal-button" onClick={() => setIsAnswerRevealed(true)}>
                    Reveal answer
                  </button>
                )}
                <div className="grade-row session-grades">
                  {(["again", "hard", "good", "easy"] as const).map((grade) => (
                    <button key={grade} type="button" onClick={() => handleSessionGrade(grade)} disabled={!isAnswerRevealed || isGrading}>
                      {isGrading ? "Saving..." : grade}
                    </button>
                  ))}
                </div>
              </article>
            ) : null}
            {sessionSummary ? (
              <div className="session-summary">
                <strong>Session complete.</strong>
                <span>
                  Reviewed {sessionSummary.reviewed} card{sessionSummary.reviewed === 1 ? "" : "s"} with {sessionSummary.again} marked again.
                </span>
                {sessionSummary.lastNextDue ? <span>Last card returns {formatDate(sessionSummary.lastNextDue)}.</span> : null}
              </div>
            ) : null}
            <div className="review-list">
              {due.map(({ item, state }) => (
                <article className="review-card" key={item.id}>
                  <div>
                    <p className="term">{item.term}</p>
                    <p>{item.meaning || "Meaning not added yet."}</p>
                    <small>Next due: {formatDate(state.next_due_at)}</small>
                  </div>
                  <div className="grade-row">
                    {(["again", "hard", "good", "easy"] as const).map((grade) => (
                      <button key={grade} type="button" onClick={() => handleGrade(item.id, grade)} disabled={isGrading || sessionActive}>
                        {isGrading ? "Saving..." : grade}
                      </button>
                    ))}
                  </div>
                </article>
              ))}
              {due.length === 0 ? (
                <div className="empty-state">
                  <strong>All caught up.</strong>
                  <span>No cards are due right now.</span>
                </div>
              ) : null}
            </div>
          </section>
        ) : null}

        {activeSection === "add" ? (
          <section className="panel quick-add">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Capture</p>
                <h2>Quick add</h2>
                <small>Only the word or phrase is required. Add meaning later if you are moving fast.</small>
              </div>
            </div>
            <form className="stack" onSubmit={handleCreateVocab}>
              <div className="quick-capture-line">
                <input
                  ref={termInputRef}
                  value={form.term}
                  placeholder="Word or phrase"
                  onChange={(event) => setForm({ ...form, term: event.target.value })}
                />
                <select value={form.kind} onChange={(event) => setForm({ ...form, kind: event.target.value as "word" | "phrase" })}>
                  <option value="word">Word</option>
                  <option value="phrase">Phrase</option>
                </select>
              </div>
              <details className="optional-fields">
                <summary>Meaning, example, and notes</summary>
                <textarea value={form.meaning} placeholder="Meaning" onChange={(event) => setForm({ ...form, meaning: event.target.value })} />
                <textarea
                  value={form.example_sentence}
                  placeholder="Example sentence"
                  onChange={(event) => setForm({ ...form, example_sentence: event.target.value })}
                />
                <textarea value={form.notes} placeholder="Notes" onChange={(event) => setForm({ ...form, notes: event.target.value })} />
              </details>
              <div className="action-row quick-actions">
                <button type="submit" name="save-add" disabled={isSaving || !form.term.trim()}>
                  {isSaving ? "Saving..." : "Save + add another"}
                </button>
                <button type="submit" name="save-review" className="ghost-button" disabled={isSaving || !form.term.trim()}>
                  Save + review
                </button>
              </div>
              {lastCreatedTerm ? <p className="save-confirmation">Saved "{lastCreatedTerm}". Ready for the next one.</p> : null}
            </form>

            <form className="bulk-import" onSubmit={handleBulkImport}>
              <div className="section-heading import-heading">
                <div>
                  <p className="eyebrow">Batch capture</p>
                  <h2>Bulk import</h2>
                  <small>Paste one card per line. Use "term - meaning", "term: meaning", or just the term.</small>
                </div>
                <span className="import-count">{parsedImportCards.length} cards</span>
              </div>
              <textarea
                className="bulk-textarea"
                value={bulkText}
                placeholder={"abandon - to leave behind\nmeticulous: very careful\nmake up"}
                onChange={(event) => setBulkText(event.target.value)}
              />
              <div className="import-preview">
                {parsedImportCards.length === 0 ? (
                  <span className="muted">Parsed cards will appear here before import.</span>
                ) : (
                  parsedImportCards.slice(0, 8).map((card, index) => (
                    <article key={`${card.term}-${index}`} className="import-preview-card">
                      <strong>{card.term}</strong>
                      <span>{card.meaning || "Meaning can be added later."}</span>
                    </article>
                  ))
                )}
                {parsedImportCards.length > 8 ? <span className="muted">+ {parsedImportCards.length - 8} more</span> : null}
              </div>
              <button type="submit" disabled={isSaving || parsedImportCards.length === 0}>
                {isSaving ? "Importing..." : `Import ${parsedImportCards.length || ""} cards`}
              </button>
              {lastImportCount ? (
                <p className="save-confirmation">
                  Imported {lastImportCount} card{lastImportCount === 1 ? "" : "s"}.
                </p>
              ) : null}
            </form>
          </section>
        ) : null}

        {activeSection === "history" ? (
          <section className="panel history-panel">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Progress</p>
                <h2>Recent reviews</h2>
              </div>
            </div>
            <div className="history-grid">
              {history.map((entry) => (
                <button
                  className={selectedHistory?.log.id === entry.log.id ? "history-card selected" : "history-card"}
                  key={entry.log.id}
                  type="button"
                  onClick={() => toggleHistoryDetail(entry)}
                >
                  {entry.item.archived_at ? <span className="archive-corner">Archived</span> : null}
                  <span className="history-word">{entry.item.term}</span>
                  <span className="history-status">{entry.state.status}</span>
                  <span className="history-preview">{entry.item.meaning || "Meaning not added yet."}</span>
                </button>
              ))}
            </div>
            {selectedHistory ? (
              <article className="history-detail">
                <div className="section-heading">
                  <div>
                    <p className="eyebrow">Full info</p>
                    <h2>{selectedHistory.item.term}</h2>
                  </div>
                  <button type="button" className="ghost-button compact-button" onClick={() => setSelectedHistory(null)}>
                    Close
                  </button>
                </div>
                <p>{selectedHistory.item.meaning || "Meaning not added yet."}</p>
                {selectedHistory.item.example_sentence ? <small>{selectedHistory.item.example_sentence}</small> : null}
                {selectedHistory.item.notes ? <small className="notes">Notes: {selectedHistory.item.notes}</small> : null}
                <div className="meta">
                  <span>Grade: {selectedHistory.log.grade}</span>
                  <span>Status: {selectedHistory.state.status}</span>
                  <span>Reviewed {formatDate(selectedHistory.log.reviewed_at)}</span>
                  <span>Next due {formatDate(selectedHistory.state.next_due_at)}</span>
                  {selectedHistory.item.archived_at ? <span className="archived-badge">Archived</span> : null}
                </div>
              </article>
            ) : null}
            {history.length === 0 ? (
              <div className="empty-state">
                <strong>No reviews yet.</strong>
                <span>Review a due card and it will appear here.</span>
              </div>
            ) : null}
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
                onChange={(event) => setQuery(event.target.value)}
              />
              <div className="filter-chips" aria-label="Filter cards by status">
                {(["all", "new", "learning", "review"] as const).map((status) => (
                  <button
                    key={status}
                    type="button"
                    className={statusFilter === status ? "chip active" : "chip"}
                    onClick={() => setStatusFilter(status)}
                  >
                    {status}
                  </button>
                ))}
              </div>
            </div>

            <div className="library">
              {visibleVocab.map(({ item, state }) => (
                <article className={editingID === item.id ? "library-card editing" : "library-card"} key={item.id}>
                  {editingID === item.id ? (
                    <form className="edit-form" onSubmit={handleSaveEdit}>
                      <div className="edit-grid">
                        <input value={editDraft.term} onChange={(event) => setEditDraft({ ...editDraft, term: event.target.value })} />
                        <select
                          value={editDraft.kind}
                          onChange={(event) => setEditDraft({ ...editDraft, kind: event.target.value as "word" | "phrase" })}
                        >
                          <option value="word">Word</option>
                          <option value="phrase">Phrase</option>
                        </select>
                      </div>
                      <textarea value={editDraft.meaning} onChange={(event) => setEditDraft({ ...editDraft, meaning: event.target.value })} />
                      <textarea
                        value={editDraft.example_sentence}
                        onChange={(event) => setEditDraft({ ...editDraft, example_sentence: event.target.value })}
                      />
                      <textarea value={editDraft.notes} onChange={(event) => setEditDraft({ ...editDraft, notes: event.target.value })} />
                      <div className="action-row">
                        <button type="submit" disabled={isSaving}>{isSaving ? "Saving..." : "Save changes"}</button>
                        <button type="button" className="ghost-button" onClick={() => setEditingID("")}>Cancel</button>
                      </div>
                    </form>
                  ) : (
                    <>
                      <div className="card-copy">
                        <div>
                          <p className="term">{item.term}</p>
                          <p>{item.meaning || "Meaning not added yet."}</p>
                          {item.example_sentence ? <small>{item.example_sentence}</small> : null}
                          {item.notes ? <small className="notes">Notes: {item.notes}</small> : null}
                        </div>
                        <div className="meta">
                          <span>{item.kind}</span>
                          <span>{state.status}</span>
                          <span>{state.interval_days}d interval</span>
                          <span>Created {formatDate(item.created_at)}</span>
                        </div>
                      </div>
                      <div className="action-row">
                        <button type="button" className="ghost-button" onClick={() => startEditing(item)}>Edit</button>
                        <button type="button" className="danger-button" onClick={() => handleArchive(item.id)} disabled={isSaving}>
                          Archive
                        </button>
                      </div>
                    </>
                  )}
                </article>
              ))}
            </div>

            {visibleVocab.length === 0 ? (
              <div className="empty-state spacious">
                <strong>{vocab.length === 0 ? "No cards yet." : "No matching cards."}</strong>
                <span>{vocab.length === 0 ? "Add your first word above." : "Try another search or status filter."}</span>
              </div>
            ) : null}
          </section>
        ) : null}

        {error ? <p className="error floating-error">{error}</p> : null}
      </section>
    </main>
  );
}
