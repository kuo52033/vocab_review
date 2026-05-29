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

type AutocompleteItem = {
  term: string;
  meaning: string;
  chinese: string;
  example_sentence: string;
  part_of_speech: string;
};

type AutocompleteResult = AutocompleteItem & {
  error: string;
};

const QUEUE_KEY = "queuedCaptures";
const AUTOCOMPLETE_RUNNING_KEY = "autocompleteQueueRunning";
const IMPORT_RUNNING_KEY = "importQueueRunning";
const API_URL = (import.meta.env.VITE_API_URL as string | undefined) ?? "http://localhost:8080";
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

function normalizePartOfSpeech(value: string) {
  const normalized = value.trim().toLowerCase().replace(/[\s-]+/g, "_");
  if (allowedPartsOfSpeech.has(normalized)) return normalized;
  return normalized ? "other" : "";
}

async function request<T>(path: string, sessionToken: string, init?: RequestInit): Promise<T> {
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

async function autocompleteQueue() {
  const stored = await chrome.storage.local.get([QUEUE_KEY, "sessionToken", AUTOCOMPLETE_RUNNING_KEY]);
  if (stored[AUTOCOMPLETE_RUNNING_KEY]) {
    return { message: "Auto-fill is already running." };
  }

  const queuedCaptures = (stored[QUEUE_KEY] ?? []) as QueuedCapture[];
  if (queuedCaptures.length === 0) {
    return { message: "No queued words to fill." };
  }
  if (!stored.sessionToken) {
    throw new Error("Sign in again.");
  }

  await chrome.storage.local.set({ [AUTOCOMPLETE_RUNNING_KEY]: true });
  try {
    const items: AutocompleteItem[] = queuedCaptures.map((capture) => ({
      term: capture.term,
      meaning: capture.meaning ?? "",
      chinese: capture.chinese ?? "",
      example_sentence: capture.example_sentence ?? "",
      part_of_speech: capture.part_of_speech ?? ""
    }));
    const response = await request<{ items: AutocompleteResult[] }>("/vocab/autocomplete", stored.sessionToken, {
      method: "POST",
      body: JSON.stringify({ items })
    });
    const enrichedQueue = queuedCaptures.map((capture, index) => {
      const result = response.items[index];
      if (!result) return capture;
      return {
        ...capture,
        meaning: capture.meaning || result.meaning || "",
        chinese: capture.chinese || result.chinese || "",
        example_sentence: capture.example_sentence || result.example_sentence || "",
        part_of_speech: capture.part_of_speech || normalizePartOfSpeech(result.part_of_speech || "")
      };
    });
    await chrome.storage.local.set({ [QUEUE_KEY]: enrichedQueue });
    return { message: "Auto-filled queued captures." };
  } finally {
    await chrome.storage.local.set({ [AUTOCOMPLETE_RUNNING_KEY]: false });
  }
}

async function updateQueueBadge(length: number) {
  await chrome.action.setBadgeText({ text: length ? String(Math.min(length, 99)) : "" });
  if (length) {
    await chrome.action.setBadgeBackgroundColor({ color: "#657b5f" });
  }
}

async function importQueue() {
  const stored = await chrome.storage.local.get([QUEUE_KEY, "sessionToken", IMPORT_RUNNING_KEY]);
  if (stored[IMPORT_RUNNING_KEY]) {
    return { message: "Import is already running." };
  }

  let queuedCaptures = (stored[QUEUE_KEY] ?? []) as QueuedCapture[];
  if (queuedCaptures.length === 0) {
    return { message: "No queued words to import." };
  }
  if (!stored.sessionToken) {
    throw new Error("Sign in again.");
  }

  await chrome.storage.local.set({ [IMPORT_RUNNING_KEY]: true });
  let importedCount = 0;
  try {
    while (queuedCaptures.length > 0) {
      const capture = queuedCaptures[0];
      await request("/captures", stored.sessionToken, {
        method: "POST",
        body: JSON.stringify({
          term: capture.term,
          meaning: capture.meaning ?? "",
          chinese: capture.chinese ?? "",
          example_sentence: capture.example_sentence ?? "",
          part_of_speech: capture.part_of_speech ?? "",
          selection: capture.selection,
          page_title: capture.page_title,
          page_url: capture.page_url
        })
      });
      importedCount += 1;
      queuedCaptures = queuedCaptures.slice(1);
      await chrome.storage.local.set({ [QUEUE_KEY]: queuedCaptures });
      await updateQueueBadge(queuedCaptures.length);
    }
    return { message: `Imported ${importedCount} ${importedCount === 1 ? "card" : "cards"}.` };
  } finally {
    await chrome.storage.local.set({ [IMPORT_RUNNING_KEY]: false });
  }
}

async function ensureContextMenu() {
  await chrome.contextMenus.removeAll();
  chrome.contextMenus.create({
    id: "save-selection",
    title: "Save to Vocab Review",
    contexts: ["selection"]
  });
}

void ensureContextMenu();

chrome.runtime.onInstalled.addListener(() => {
  void ensureContextMenu();
});

chrome.runtime.onStartup.addListener(() => {
  void ensureContextMenu();
});

chrome.contextMenus.onClicked.addListener(async (info, tab) => {
  if (info.menuItemId !== "save-selection") return;
  const selectedText = (info.selectionText ?? "").trim();
  if (!selectedText) return;

  const stored = await chrome.storage.local.get(QUEUE_KEY);
  const queuedCaptures = (stored[QUEUE_KEY] ?? []) as QueuedCapture[];
  const nextCapture: QueuedCapture = {
    id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
    term: selectedText,
    selection: selectedText,
    page_title: tab?.title ?? "",
    page_url: tab?.url ?? "",
    created_at: new Date().toISOString()
  };

  await chrome.storage.local.set({
    [QUEUE_KEY]: [nextCapture, ...queuedCaptures].slice(0, 100)
  });
  await chrome.action.setBadgeText({ text: String(Math.min(queuedCaptures.length + 1, 99)) });
  await chrome.action.setBadgeBackgroundColor({ color: "#657b5f" });
});

chrome.runtime.onMessage.addListener((message: { type?: string }, _sender, sendResponse) => {
  if (message.type !== "autocomplete-queue" && message.type !== "import-queue") return false;

  const action = message.type === "autocomplete-queue" ? autocompleteQueue : importQueue;
  void action()
    .then((result) => sendResponse({ ok: true, message: result.message }))
    .catch((error) => sendResponse({ ok: false, error: (error as Error).message }));

  return true;
});
