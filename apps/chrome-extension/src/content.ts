chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message.type !== "GET_SELECTION") return;
  sendResponse({
    selection: window.getSelection()?.toString() ?? "",
    title: document.title,
    url: window.location.href
  });
});
