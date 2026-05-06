import { FormEvent, useEffect, useState } from "react";
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

export function App() {
  const [auth, setAuth] = useState<AuthState>({ email: "", token: localStorage.getItem("session_token") ?? "" });
  const [vocab, setVocab] = useState<VocabWithState[]>([]);
  const [due, setDue] = useState<VocabWithState[]>([]);
  const [history, setHistory] = useState<ReviewHistoryEntry[]>([]);
  const [stats, setStats] = useState<ReviewStats>(emptyStats);
  const [jobs, setJobs] = useState<Array<{ id: string; vocab_item_id: string; status: string; scheduled_at: string }>>([]);
  const [form, setForm] = useState(emptyForm);
  const [editingID, setEditingID] = useState("");
  const [editDraft, setEditDraft] = useState<CardDraft>(emptyForm);
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [activeSection, setActiveSection] = useState<ActiveSection>("review");
  const [selectedHistory, setSelectedHistory] = useState<ReviewHistoryEntry | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
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

  async function handleCreateVocab(event: FormEvent) {
    event.preventDefault();
    setIsSaving(true);
    try {
      await createVocab(form);
      setForm(emptyForm);
      await refresh();
    } catch (err) {
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
    try {
      await gradeReview(id, grade);
      await refresh();
    } catch (err) {
      setError((err as Error).message);
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
            </div>
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
                      <button key={grade} type="button" onClick={() => handleGrade(item.id, grade)}>
                        {grade}
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
              </div>
            </div>
            <form className="stack" onSubmit={handleCreateVocab}>
              <input value={form.term} placeholder="Word or phrase" onChange={(event) => setForm({ ...form, term: event.target.value })} />
              <select value={form.kind} onChange={(event) => setForm({ ...form, kind: event.target.value as "word" | "phrase" })}>
                <option value="word">Word</option>
                <option value="phrase">Phrase</option>
              </select>
              <textarea value={form.meaning} placeholder="Meaning" onChange={(event) => setForm({ ...form, meaning: event.target.value })} />
              <textarea
                value={form.example_sentence}
                placeholder="Example sentence"
                onChange={(event) => setForm({ ...form, example_sentence: event.target.value })}
              />
              <textarea value={form.notes} placeholder="Notes" onChange={(event) => setForm({ ...form, notes: event.target.value })} />
              <button type="submit" disabled={isSaving}>{isSaving ? "Saving..." : "Save card"}</button>
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
