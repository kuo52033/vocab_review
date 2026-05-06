import React, { FormEvent, useEffect, useRef, useState } from "react";
import ReactDOM from "react-dom/client";
import "./popup.css";

type Draft = {
  term: string;
  meaning: string;
  example_sentence: string;
  selection: string;
  page_title: string;
  page_url: string;
};

type MagicLink = {
  token: string;
  verification_url: string;
  expires_at: string;
};

const API_URL = "http://localhost:8080";

const emptyDraft: Draft = {
  term: "",
  meaning: "",
  example_sentence: "",
  selection: "",
  page_title: "",
  page_url: ""
};

function Popup() {
  const termInputRef = useRef<HTMLInputElement>(null);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [email, setEmail] = useState("");
  const [magicToken, setMagicToken] = useState("");
  const [magicLink, setMagicLink] = useState<MagicLink | null>(null);
  const [sessionToken, setSessionToken] = useState("");
  const [status, setStatus] = useState("");
  const [lastSavedTerm, setLastSavedTerm] = useState("");
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    chrome.storage.local.get(["draftSelection", "draftPageURL", "draftPageTitle", "sessionToken", "email"], async (stored) => {
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
    });
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
