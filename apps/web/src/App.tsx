import { FormEvent, useEffect, useState } from "react";
import {
  createVocab,
  gradeReview,
  listDue,
  listNotificationJobs,
  listVocab,
  requestMagicLink,
  setToken,
  verifyMagicLink,
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

const emptyForm: CardDraft = {
  term: "",
  kind: "word",
  meaning: "",
  example_sentence: "",
  notes: ""
};

export function App() {
  const [auth, setAuth] = useState<AuthState>({ email: "", token: localStorage.getItem("session_token") ?? "" });
  const [vocab, setVocab] = useState<VocabWithState[]>([]);
  const [due, setDue] = useState<VocabWithState[]>([]);
  const [jobs, setJobs] = useState<Array<{ id: string; vocab_item_id: string; status: string; scheduled_at: string }>>([]);
  const [form, setForm] = useState(emptyForm);
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

  async function refresh() {
    const [vocabResponse, dueResponse, jobsResponse] = await Promise.all([
      listVocab(),
      listDue(),
      listNotificationJobs()
    ]);
    setVocab(vocabResponse.items);
    setDue(dueResponse.items);
    setJobs(jobsResponse.items);
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
    try {
      await createVocab(form);
      setForm(emptyForm);
      await refresh();
    } catch (err) {
      setError((err as Error).message);
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

  if (!auth.token) {
    return (
      <main className="shell">
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
    <main className="shell">
      <header className="hero">
        <div>
          <p className="eyebrow">Personal English memory system</p>
          <h1>Review what is due before it fades.</h1>
        </div>
        <div className="stats">
          <article>
            <strong>{due.length}</strong>
            <span>Due now</span>
          </article>
          <article>
            <strong>{vocab.length}</strong>
            <span>Total cards</span>
          </article>
          <article>
            <strong>{jobs.length}</strong>
            <span>Queued reminders</span>
          </article>
        </div>
      </header>

      <section className="grid">
        <section className="panel">
          <h2>Quick add</h2>
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
            <button type="submit">Save card</button>
          </form>
        </section>

        <section className="panel">
          <h2>Due now</h2>
          <div className="review-list">
            {due.map(({ item, state }) => (
              <article className="review-card" key={item.id}>
                <div>
                  <p className="term">{item.term}</p>
                  <p>{item.meaning || "Meaning not added yet."}</p>
                  <small>Next due: {new Date(state.next_due_at).toLocaleString()}</small>
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
            {due.length === 0 ? <p className="muted">No cards are due right now.</p> : null}
          </div>
        </section>
      </section>

      <section className="panel">
        <h2>Library</h2>
        <div className="library">
          {vocab.map(({ item, state }) => (
            <article className="library-card" key={item.id}>
              <div>
                <p className="term">{item.term}</p>
                <p>{item.meaning}</p>
                <small>{item.example_sentence}</small>
              </div>
              <div className="meta">
                <span>{item.kind}</span>
                <span>{state.status}</span>
                <span>{state.interval_days}d</span>
              </div>
            </article>
          ))}
        </div>
      </section>

      {error ? <p className="error">{error}</p> : null}
    </main>
  );
}
