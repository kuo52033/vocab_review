type QueuedCapture = {
  id: string;
  term: string;
  selection: string;
  page_title: string;
  page_url: string;
  created_at: string;
};

const QUEUE_KEY = "queuedCaptures";

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
    [QUEUE_KEY]: [nextCapture, ...queuedCaptures].slice(0, 100),
    draftSelection: selectedText,
    draftPageURL: tab?.url ?? "",
    draftPageTitle: tab?.title ?? ""
  });
  await chrome.action.setBadgeText({ text: String(Math.min(queuedCaptures.length + 1, 99)) });
  await chrome.action.setBadgeBackgroundColor({ color: "#657b5f" });
});
