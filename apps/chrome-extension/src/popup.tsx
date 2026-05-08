import React, { FormEvent, useEffect, useRef, useState } from "react";
import ReactDOM from "react-dom/client";
import "./popup.css";

type Draft = {
  term: string;
  meaning: string;
  example_sentence: string;
  part_of_speech: string;
  selection: string;
  page_title: string;
  page_url: string;
};

type MagicLink = {
  token: string;
  verification_url: string;
  expires_at: string;
};

type QueuedCapture = {
  id: string;
  term: string;
  meaning?: string;
  example_sentence?: string;
  part_of_speech?: string;
  selection: string;
  page_title: string;
  page_url: string;
  created_at: string;
};

const API_URL = "http://localhost:8080";
const QUEUE_KEY = "queuedCaptures";
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

function newQueuedCapture(term: string, pageTitle = "", pageURL = ""): QueuedCapture {
  return {
    id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
    term,
    selection: term,
    page_title: pageTitle,
    page_url: pageURL,
    created_at: new Date().toISOString()
  };
}

const emptyDraft: Draft = {
  term: "",
  meaning: "",
  example_sentence: "",
  part_of_speech: "",
  selection: "",
  page_title: "",
  page_url: ""
};

type AutocompleteItem = {
  term: string;
  meaning: string;
  example_sentence: string;
  part_of_speech: string;
};

type AutocompleteResult = AutocompleteItem & {
  error: string;
};

function normalizePartOfSpeech(value: string) {
  const normalized = value.trim().toLowerCase().replace(/[\s-]+/g, "_");
  if (allowedPartsOfSpeech.has(normalized)) return normalized;
  return normalized ? "other" : "";
}

function Popup() {
  const termInputRef = useRef<HTMLInputElement>(null);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [email, setEmail] = useState("");
  const [magicToken, setMagicToken] = useState("");
  const [magicLink, setMagicLink] = useState<MagicLink | null>(null);
  const [sessionToken, setSessionToken] = useState("");
  const [queuedCaptures, setQueuedCaptures] = useState<QueuedCapture[]>([]);
  const [status, setStatus] = useState("");
  const [lastSavedTerm, setLastSavedTerm] = useState("");
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    chrome.storage.local.get(["draftSelection", "draftPageURL", "draftPageTitle", "sessionToken", "email", QUEUE_KEY], async (stored) => {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      const response = tab.id ? await chrome.tabs.sendMessage(tab.id, { type: "GET_SELECTION" }).catch(() => null) : null;
      const selection = stored.draftSelection || response?.selection || "";

      setDraft({
        ...emptyDraft,
        selection,
        page_title: stored.draftPageTitle || response?.title || "",
        page_url: stored.draftPageURL || response?.url || "",
        term: selection
      });
      setSessionToken(stored.sessionToken || "");
      setEmail(stored.email || "");

      const storedQueue = (stored[QUEUE_KEY] || []) as QueuedCapture[];
      if (storedQueue.length === 0 && stored.draftSelection) {
        const migratedQueue = [newQueuedCapture(stored.draftSelection, stored.draftPageTitle || "", stored.draftPageURL || "")];
        await chrome.storage.local.set({ [QUEUE_KEY]: migratedQueue });
        await chrome.action.setBadgeText({ text: "1" });
        await chrome.action.setBadgeBackgroundColor({ color: "#657b5f" });
        setQueuedCaptures(migratedQueue);
      } else {
        setQueuedCaptures(storedQueue);
        await chrome.action.setBadgeText({ text: storedQueue.length ? String(Math.min(storedQueue.length, 99)) : "" });
        if (storedQueue.length) {
          await chrome.action.setBadgeBackgroundColor({ color: "#657b5f" });
        }
      }
    });

    const handleStorageChange = (changes: Record<string, chrome.storage.StorageChange>, areaName: string) => {
      if (areaName !== "local" || !changes[QUEUE_KEY]) return;
      setQueuedCaptures(changes[QUEUE_KEY].newValue || []);
    };

    chrome.storage.onChanged.addListener(handleStorageChange);
    return () => chrome.storage.onChanged.removeListener(handleStorageChange);
  }, []);

  async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await fetch(`${API_URL}${path}`, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...(sessionToken ? { Authorization: `Bearer ${sessionToken}` } : {}),
        ...(init?.headers ?? {})
      }
    });
    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: "Request failed" }));
      throw new Error(error.error ?? "Request failed");
    }
    return response.json();
  }

  async function handleRequestLink(event: FormEvent) {
    event.preventDefault();
    setStatus("Requesting link...");
    setIsLoading(true);
    try {
      const response = await request<MagicLink>("/auth/magic-link", {
        method: "POST",
        body: JSON.stringify({ email, base_url: "http://localhost:5173" })
      });
      await chrome.storage.local.set({ email });
      setMagicLink(response);
      setMagicToken(response.token);
      setStatus("Development verification token is ready.");
    } catch (error) {
      setStatus((error as Error).message);
    } finally {
      setIsLoading(false);
    }
  }

  async function handleVerify(event: FormEvent) {
    event.preventDefault();
    setStatus("Signing in...");
    setIsLoading(true);
    try {
      const response = await request<{ session: { token: string } }>("/auth/verify", {
        method: "POST",
        body: JSON.stringify({ token: magicToken })
      });
      await chrome.storage.local.set({ sessionToken: response.session.token });
      setSessionToken(response.session.token);
      setMagicLink(null);
      setMagicToken("");
      setStatus("Signed in. Ready to capture.");
    } catch (error) {
      setStatus((error as Error).message);
    } finally {
      setIsLoading(false);
    }
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    const savedTerm = draft.term.trim();
    setStatus("Saving...");
    setIsLoading(true);
    try {
      await request("/captures", {
        method: "POST",
        body: JSON.stringify({
          term: draft.term,
          meaning: draft.meaning,
          example_sentence: draft.example_sentence,
          part_of_speech: draft.part_of_speech,
          selection: draft.selection,
          page_title: draft.page_title,
          page_url: draft.page_url
        })
      });
      await chrome.storage.local.remove(["draftSelection", "draftPageURL", "draftPageTitle"]);
      setLastSavedTerm(savedTerm);
      setStatus(`Saved "${savedTerm}". Ready for another capture.`);
      setDraft({
        ...emptyDraft,
        page_title: draft.page_title,
        page_url: draft.page_url
      });
      requestAnimationFrame(() => termInputRef.current?.focus());
    } catch (error) {
      setStatus((error as Error).message);
    } finally {
      setIsLoading(false);
    }
  }

  async function handleImportQueue() {
    if (queuedCaptures.length === 0) return;
    setStatus(`Importing ${queuedCaptures.length} captures...`);
    setIsLoading(true);
    let importedCount = 0;
    try {
      for (const capture of queuedCaptures) {
        await request("/captures", {
          method: "POST",
          body: JSON.stringify({
            term: capture.term,
            meaning: capture.meaning ?? "",
            example_sentence: capture.example_sentence ?? "",
            part_of_speech: capture.part_of_speech ?? "",
            selection: capture.selection,
            page_title: capture.page_title,
            page_url: capture.page_url
          })
        });
        importedCount += 1;
      }
      await chrome.storage.local.set({ [QUEUE_KEY]: [] });
      await chrome.action.setBadgeText({ text: "" });
      setQueuedCaptures([]);
      setStatus(`Imported ${importedCount} ${importedCount === 1 ? "card" : "cards"}.`);
    } catch (error) {
      const remaining = queuedCaptures.slice(importedCount);
      await chrome.storage.local.set({ [QUEUE_KEY]: remaining });
      await chrome.action.setBadgeText({ text: remaining.length ? String(Math.min(remaining.length, 99)) : "" });
      setQueuedCaptures(remaining);
      setStatus((error as Error).message);
    } finally {
      setIsLoading(false);
    }
  }

  async function removeQueuedCapture(id: string) {
    const nextQueue = queuedCaptures.filter((capture) => capture.id !== id);
    await chrome.storage.local.set({ [QUEUE_KEY]: nextQueue });
    await chrome.action.setBadgeText({ text: nextQueue.length ? String(Math.min(nextQueue.length, 99)) : "" });
    setQueuedCaptures(nextQueue);
  }

  async function handleAutocompleteQueue() {
    if (queuedCaptures.length === 0) return;
    setStatus(`Auto-completing ${queuedCaptures.length} queued captures...`);
    setIsLoading(true);
    try {
      const items: AutocompleteItem[] = queuedCaptures.map((capture) => ({
        term: capture.term,
        meaning: capture.meaning ?? "",
        example_sentence: capture.example_sentence ?? "",
        part_of_speech: capture.part_of_speech ?? ""
      }));
      const response = await request<{ items: AutocompleteResult[] }>("/vocab/autocomplete", {
        method: "POST",
        body: JSON.stringify({ items })
      });
      const enrichedQueue = queuedCaptures.map((capture, index) => {
        const result = response.items[index];
        if (!result) return capture;
        return {
          ...capture,
          meaning: capture.meaning || result.meaning || "",
          example_sentence: capture.example_sentence || result.example_sentence || "",
          part_of_speech: capture.part_of_speech || normalizePartOfSpeech(result.part_of_speech || "")
        };
      });
      await chrome.storage.local.set({ [QUEUE_KEY]: enrichedQueue });
      setQueuedCaptures(enrichedQueue);
      setStatus("Auto-completed queued captures. You can still edit later in the app.");
    } catch (error) {
      setStatus(`${(error as Error).message}. Queue import still works manually.`);
    } finally {
      setIsLoading(false);
    }
  }

  async function clearQueue() {
    await chrome.storage.local.set({ [QUEUE_KEY]: [] });
    await chrome.action.setBadgeText({ text: "" });
    setQueuedCaptures([]);
    setStatus("Capture queue cleared.");
  }

  async function handleSignOut() {
    await chrome.storage.local.remove("sessionToken");
    setSessionToken("");
    setStatus("Signed out.");
  }

  return (
    <main className="popup">
      <section className="hero">
        <p className="eyebrow">Vocab Review</p>
        <h1>Capture a word while it is still warm.</h1>
      </section>

      {!sessionToken ? (
        <section className="card">
          <h2>Sign in</h2>
          <form onSubmit={handleRequestLink}>
            <label>
              Email
              <input
                type="email"
                value={email}
                placeholder="you@example.com"
                onChange={(event) => setEmail(event.target.value)}
              />
            </label>
            <button type="submit" disabled={isLoading || !email.trim()}>
              {isLoading ? "Working..." : "Request magic link"}
            </button>
          </form>

          {magicLink ? (
            <form onSubmit={handleVerify} className="verify-block">
              <p className="small">Development URL</p>
              <a href={magicLink.verification_url} target="_blank" rel="noreferrer">
                {magicLink.verification_url}
              </a>
              <label>
                Verification token
                <input value={magicToken} onChange={(event) => setMagicToken(event.target.value)} />
              </label>
              <button type="submit" disabled={isLoading || !magicToken.trim()}>
                Verify token
              </button>
            </form>
          ) : null}
        </section>
      ) : (
        <section className="card">
          <div className="capture-heading">
            <div>
              <h2>Quick capture</h2>
              <p className="small">Only the word or phrase is required.</p>
            </div>
            <button type="button" className="text-button" onClick={handleSignOut}>
              Sign out
            </button>
          </div>
          <form onSubmit={handleSubmit}>
            <label>
              Word or phrase
              <input ref={termInputRef} value={draft.term} onChange={(event) => setDraft({ ...draft, term: event.target.value })} />
            </label>
            <details className="optional-fields">
              <summary>Meaning and example</summary>
              <label>
                Meaning
                <textarea value={draft.meaning} onChange={(event) => setDraft({ ...draft, meaning: event.target.value })} />
              </label>
              <label>
                Example sentence
                <textarea
                  value={draft.example_sentence}
                  onChange={(event) => setDraft({ ...draft, example_sentence: event.target.value })}
                />
              </label>
            </details>
            <details className="optional-fields" open={Boolean(draft.selection)}>
              <summary>Source context</summary>
              <label>
                Source selection
                <textarea value={draft.selection} onChange={(event) => setDraft({ ...draft, selection: event.target.value })} />
              </label>
            </details>
            {draft.page_title || draft.page_url ? (
              <div className="source">
                <strong>{draft.page_title || "Current page"}</strong>
                <span>{draft.page_url}</span>
              </div>
            ) : null}
            <button type="submit" disabled={isLoading || !draft.term.trim()}>
              {isLoading ? "Saving..." : "Save + capture another"}
            </button>
            {lastSavedTerm ? <p className="saved-note">Last saved: {lastSavedTerm}</p> : null}
          </form>

          <section className="queue-panel">
            <div className="queue-heading">
              <div>
                <h2>Bulk import queue</h2>
                <p className="small">Right-click selected words on a page, then import them here once.</p>
              </div>
              <span className="queue-count">{queuedCaptures.length}</span>
            </div>

            {queuedCaptures.length === 0 ? (
              <p className="empty-queue">No queued words yet.</p>
            ) : (
              <div className="queue-list">
                {queuedCaptures.map((capture) => (
                  <article className="queue-item" key={capture.id}>
                    <div>
                      <strong>{capture.term}</strong>
                      <span>{capture.page_title || "Current page"}</span>
                      {capture.meaning ? <span>{capture.meaning}</span> : null}
                      {capture.part_of_speech ? <span className="pos-pill">{capture.part_of_speech.replace(/_/g, " ")}</span> : null}
                    </div>
                    <button type="button" className="text-button" onClick={() => removeQueuedCapture(capture.id)} disabled={isLoading}>
                      Remove
                    </button>
                  </article>
                ))}
              </div>
            )}

            <div className="queue-actions">
              <button type="button" className="secondary-button" onClick={handleAutocompleteQueue} disabled={isLoading || queuedCaptures.length === 0}>
                {isLoading ? "Working..." : "Auto-complete"}
              </button>
              <button type="button" onClick={handleImportQueue} disabled={isLoading || queuedCaptures.length === 0}>
                {isLoading ? "Importing..." : `Import ${queuedCaptures.length || ""} cards`}
              </button>
              <button type="button" className="secondary-button" onClick={clearQueue} disabled={isLoading || queuedCaptures.length === 0}>
                Clear
              </button>
            </div>
          </section>
        </section>
      )}

      {status ? <p className="status">{status}</p> : null}
    </main>
  );
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Popup />
  </React.StrictMode>
);
