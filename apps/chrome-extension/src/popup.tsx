import React, { FormEvent, useEffect, useState } from "react";
import ReactDOM from "react-dom/client";
import "./popup.css";

type MagicLink = {
  message: string;
  token?: string;
  verification_url?: string;
  expires_at?: string;
};

type QueuedCapture = {
  id: string;
  term: string;
  meaning?: string;
  chinese?: string;
  example_sentence?: string;
  part_of_speech?: string;
  selection: string;
  page_title: string;
  page_url: string;
  created_at: string;
};

const API_URL = (import.meta.env.VITE_API_URL as string | undefined) ?? "http://localhost:8080";
const QUEUE_KEY = "queuedCaptures";
const AUTOCOMPLETE_RUNNING_KEY = "autocompleteQueueRunning";
const IMPORT_RUNNING_KEY = "importQueueRunning";

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

function Popup() {
  const [email, setEmail] = useState("");
  const [magicToken, setMagicToken] = useState("");
  const [magicLink, setMagicLink] = useState<MagicLink | null>(null);
  const [sessionToken, setSessionToken] = useState("");
  const [queuedCaptures, setQueuedCaptures] = useState<QueuedCapture[]>([]);
  const [status, setStatus] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isAutocompleteRunning, setIsAutocompleteRunning] = useState(false);
  const [isImportRunning, setIsImportRunning] = useState(false);
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false);

  useEffect(() => {
    const tokenFromURL = new URLSearchParams(window.location.search).get("token");
    if (tokenFromURL) {
      setMagicToken(tokenFromURL);
      void verifyToken(tokenFromURL);
      window.history.replaceState({}, "", window.location.pathname);
    }

    chrome.storage.local.get(
      ["draftSelection", "draftPageURL", "draftPageTitle", "sessionToken", "email", QUEUE_KEY, AUTOCOMPLETE_RUNNING_KEY, IMPORT_RUNNING_KEY],
      async (stored) => {
        if (!tokenFromURL) {
          setSessionToken(stored.sessionToken || "");
        }
        setEmail(stored.email || "");
        setIsAutocompleteRunning(Boolean(stored[AUTOCOMPLETE_RUNNING_KEY]));
        setIsImportRunning(Boolean(stored[IMPORT_RUNNING_KEY]));

        const storedQueue = (stored[QUEUE_KEY] || []) as QueuedCapture[];
        if (storedQueue.length === 0 && stored.draftSelection) {
          const migratedQueue = [newQueuedCapture(stored.draftSelection, stored.draftPageTitle || "", stored.draftPageURL || "")];
          await chrome.storage.local.set({ [QUEUE_KEY]: migratedQueue });
          await chrome.storage.local.remove(["draftSelection", "draftPageURL", "draftPageTitle"]);
          await chrome.action.setBadgeText({ text: "1" });
          await chrome.action.setBadgeBackgroundColor({ color: "#ff9494" });
          setQueuedCaptures(migratedQueue);
        } else {
          setQueuedCaptures(storedQueue);
          await chrome.action.setBadgeText({ text: storedQueue.length ? String(Math.min(storedQueue.length, 99)) : "" });
          if (storedQueue.length) {
            await chrome.action.setBadgeBackgroundColor({ color: "#ff9494" });
          }
        }
      }
    );

    const handleStorageChange = (changes: Record<string, chrome.storage.StorageChange>, areaName: string) => {
      if (areaName !== "local") return;
      if (changes[QUEUE_KEY]) {
        setQueuedCaptures(changes[QUEUE_KEY].newValue || []);
      }
      if (changes[AUTOCOMPLETE_RUNNING_KEY]) {
        setIsAutocompleteRunning(Boolean(changes[AUTOCOMPLETE_RUNNING_KEY].newValue));
      }
      if (changes[IMPORT_RUNNING_KEY]) {
        setIsImportRunning(Boolean(changes[IMPORT_RUNNING_KEY].newValue));
      }
    };

    chrome.storage.onChanged.addListener(handleStorageChange);
    return () => chrome.storage.onChanged.removeListener(handleStorageChange);
  }, []);

  useEffect(() => {
    if (!status) return;
    const timeoutID = window.setTimeout(() => setStatus(""), 3000);
    return () => window.clearTimeout(timeoutID);
  }, [status]);

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
        body: JSON.stringify({ email, base_url: chrome.runtime.getURL("popup.html"), client: "chrome_extension" })
      });
      await chrome.storage.local.set({ email });
      setMagicLink(response);
      setMagicToken(response.token ?? "");
      setStatus(response.token ? "Development verification token is ready." : response.message);
    } catch (error) {
      setStatus((error as Error).message);
    } finally {
      setIsLoading(false);
    }
  }

  async function handleVerify(event: FormEvent) {
    event.preventDefault();
    await verifyToken(magicToken);
  }

  async function verifyToken(token: string) {
    setStatus("Signing in...");
    setIsLoading(true);
    try {
      const response = await request<{ session: { token: string } }>("/auth/verify", {
        method: "POST",
        body: JSON.stringify({ token })
      });
      await chrome.storage.local.set({ sessionToken: response.session.token });
      setSessionToken(response.session.token);
      setMagicLink(null);
      setMagicToken("");
      setStatus("Signed in. Ready to import queued captures.");
    } catch (error) {
      setStatus((error as Error).message);
    } finally {
      setIsLoading(false);
    }
  }

  async function handleImportQueue() {
    if (queuedCaptures.length === 0) return;
    setStatus(`Importing ${queuedCaptures.length} captures...`);
    setIsImportRunning(true);
    try {
      const response = await chrome.runtime.sendMessage<{ type: string }, { ok: boolean; message?: string; error?: string }>({
        type: "import-queue"
      });
      if (!response?.ok) {
        throw new Error(response?.error ?? "Import failed");
      }
      setStatus(response.message ?? "Imported queued captures.");
    } catch (error) {
      setStatus((error as Error).message);
    } finally {
      setIsImportRunning(false);
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
    setStatus(`Auto-filling ${queuedCaptures.length} queued captures...`);
    setIsAutocompleteRunning(true);
    try {
      const response = await chrome.runtime.sendMessage<{ type: string }, { ok: boolean; message?: string; error?: string }>({
        type: "autocomplete-queue"
      });
      if (!response?.ok) {
        throw new Error(response?.error ?? "Auto-fill failed");
      }
      setStatus(response.message ?? "Auto-filled queued captures.");
    } catch (error) {
      setStatus(`${(error as Error).message}. Queue import still works manually.`);
    } finally {
      setIsAutocompleteRunning(false);
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
    setIsUserMenuOpen(false);
    setSessionToken("");
    setStatus("Signed out.");
  }

  function userInitial() {
    return (email.trim()[0] || "U").toUpperCase();
  }

  function sourceLabel(capture: QueuedCapture) {
    if (capture.page_url) {
      try {
        return new URL(capture.page_url).hostname;
      } catch {
        return capture.page_url;
      }
    }
    return capture.page_title || "No source URL";
  }

  function isCaptureReady(capture: QueuedCapture) {
    return Boolean(capture.meaning?.trim() && capture.chinese?.trim());
  }

  const isBusy = isLoading || isAutocompleteRunning || isImportRunning;

  return (
    <main className="popup-frame">
      <header className="ext-header">
        <div className="ext-logo">
          <div className="ext-mark" aria-hidden="true">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none">
              <path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H11v17H6.5A2.5 2.5 0 0 0 4 22.5v-17Z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
              <path d="M20 5.5A2.5 2.5 0 0 0 17.5 3H13v17h4.5a2.5 2.5 0 0 1 2.5 2.5v-17Z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
            </svg>
          </div>
          <div className="ext-name">VocabReview</div>
        </div>
        {sessionToken ? (
          <div className="user-menu">
            <button
              type="button"
              className="user-avatar"
              onClick={() => setIsUserMenuOpen((isOpen) => !isOpen)}
              aria-label="Open user menu"
              aria-expanded={isUserMenuOpen}
            >
              {userInitial()}
            </button>
            {isUserMenuOpen ? (
              <div className="user-dropdown">
                <button type="button" onClick={handleSignOut}>
                  Log out
                </button>
              </div>
            ) : null}
          </div>
        ) : null}
      </header>

      {!sessionToken ? (
        <section className="signin-panel">
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
            <button type="submit" disabled={isBusy || !email.trim()}>
              {isLoading ? "Working..." : "Request magic link"}
            </button>
          </form>

          {magicLink ? (
            <form onSubmit={handleVerify} className="verify-block">
              {magicLink.verification_url ? (
                <>
                  <p className="small">Development URL</p>
                  <a href={magicLink.verification_url} target="_blank" rel="noreferrer">
                    {magicLink.verification_url}
                  </a>
                </>
              ) : (
                <p className="small">{magicLink.message}</p>
              )}
              <label>
                Verification token
                <input value={magicToken} onChange={(event) => setMagicToken(event.target.value)} />
              </label>
              <button type="submit" disabled={isBusy || !magicToken.trim()}>
                Verify token
              </button>
            </form>
          ) : null}
        </section>
      ) : (
        <>
          <section className="hint-bar">
            <span>Right-click selected words to capture them here</span>
          </section>

          <section className="ai-row">
            <div className="ai-left">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M12 3l1.88 5.76a1 1 0 0 0 .95.69H21l-4.94 3.58a1 1 0 0 0-.36 1.12L17.56 20 12 16.18 6.44 20l1.86-5.85a1 1 0 0 0-.36-1.12L3 9.45h6.17a1 1 0 0 0 .95-.69L12 3z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              <span>Auto-fill definitions</span>
            </div>
            <button type="button" className="ai-button" onClick={handleAutocompleteQueue} disabled={isBusy || queuedCaptures.length === 0}>
              {isAutocompleteRunning ? "Working..." : "Fill missing"}
            </button>
          </section>

          <section className="word-list">
            {queuedCaptures.length === 0 ? (
              <p className="empty-queue">No words captured yet</p>
            ) : (
              queuedCaptures.map((capture) => {
                const ready = isCaptureReady(capture);
                return (
                  <article className="word-item" key={capture.id}>
                    <div className="word-body">
                      <div className="word-top">
                        <strong className="word-term">{capture.term}</strong>
                        {capture.part_of_speech ? <span className="pos-pill">{capture.part_of_speech.replace(/_/g, " ")}</span> : null}
                      </div>
                      {capture.meaning ? <p className="word-meaning">{capture.meaning}</p> : <p className="word-meaning word-missing">No definition yet</p>}
                      {capture.chinese ? <p className="word-chinese">{capture.chinese}</p> : null}
                      <p className="word-source">{sourceLabel(capture)}</p>
                    </div>
                    <div className="word-side">
                      <button
                        type="button"
                        className="remove-button"
                        onClick={() => removeQueuedCapture(capture.id)}
                        disabled={isBusy}
                        aria-label={`Remove ${capture.term}`}
                      >
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                          <path d="M3 6h18M9 6V4h6v2m4 0-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6m5 5v6m4-6v6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
                        </svg>
                      </button>
                      <span className={ready ? "word-status ready" : "word-status missing"}>{ready ? "Ready" : "Missing"}</span>
                    </div>
                  </article>
                );
              })
            )}
          </section>

          <footer className="footer-actions">
            <button type="button" className="clear-button" onClick={clearQueue} disabled={isBusy || queuedCaptures.length === 0}>
              Clear all
            </button>
            <button type="button" className="import-button" onClick={handleImportQueue} disabled={isBusy || queuedCaptures.length === 0}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4m4-5 5 5 5-5M12 15V3" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              <span>{isImportRunning ? "Importing..." : `Import ${queuedCaptures.length || ""} cards`}</span>
            </button>
          </footer>
        </>
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
