import React, { FormEvent, useEffect, useState } from "react";
import ReactDOM from "react-dom/client";
import "./popup.css";

type Draft = {
  term: string;
  meaning: string;
  example_sentence: string;
  selection: string;
  page_title: string;
  page_url: string;
  token: string;
};

const API_URL = "http://localhost:8080";

function Popup() {
  const [draft, setDraft] = useState<Draft>({
    term: "",
    meaning: "",
    example_sentence: "",
    selection: "",
    page_title: "",
    page_url: "",
    token: ""
  });
  const [status, setStatus] = useState("");

  useEffect(() => {
    chrome.storage.local.get(["draftSelection", "draftPageURL", "draftPageTitle", "sessionToken"], async (stored) => {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      const response = await chrome.tabs.sendMessage(tab.id!, { type: "GET_SELECTION" }).catch(() => null);
      setDraft((current) => ({
        ...current,
        selection: stored.draftSelection || response?.selection || "",
        page_title: stored.draftPageTitle || response?.title || "",
        page_url: stored.draftPageURL || response?.url || "",
        term: stored.draftSelection || response?.selection || "",
        token: stored.sessionToken || ""
      }));
    });
  }, []);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    setStatus("Saving...");
    try {
      await chrome.storage.local.set({ sessionToken: draft.token });
      const response = await fetch(`${API_URL}/captures`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${draft.token}`
        },
        body: JSON.stringify({
          term: draft.term,
          meaning: draft.meaning,
          example_sentence: draft.example_sentence,
          selection: draft.selection,
          page_title: draft.page_title,
          page_url: draft.page_url
        })
      });
      if (!response.ok) {
        const error = await response.json().catch(() => ({ error: "Capture failed" }));
        throw new Error(error.error ?? "Capture failed");
      }
      setStatus("Saved to your review queue.");
      setDraft((current) => ({ ...current, meaning: "", example_sentence: "" }));
    } catch (error) {
      setStatus((error as Error).message);
    }
  }

  return (
    <main className="popup">
      <h1>Quick Capture</h1>
      <form onSubmit={handleSubmit}>
        <label>
          Session token
          <input value={draft.token} onChange={(event) => setDraft({ ...draft, token: event.target.value })} />
        </label>
        <label>
          Word or phrase
          <input value={draft.term} onChange={(event) => setDraft({ ...draft, term: event.target.value })} />
        </label>
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
        <label>
          Selection
          <textarea value={draft.selection} onChange={(event) => setDraft({ ...draft, selection: event.target.value })} />
        </label>
        <button type="submit">Save card</button>
      </form>
      {status ? <p>{status}</p> : null}
    </main>
  );
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Popup />
  </React.StrictMode>
);
