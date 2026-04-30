chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: "save-selection",
    title: "Save selection to Vocab Review",
    contexts: ["selection"]
  });
});

chrome.contextMenus.onClicked.addListener(async (info, tab) => {
  if (info.menuItemId !== "save-selection") return;
  await chrome.storage.local.set({
    draftSelection: info.selectionText ?? "",
    draftPageURL: tab?.url ?? "",
    draftPageTitle: tab?.title ?? ""
  });
});
