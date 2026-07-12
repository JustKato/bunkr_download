export function initContextMenu(menuEl, handlers) {
  let activeIndex = null;

  function hide() {
    menuEl.hidden = true;
    activeIndex = null;
  }

  function show(clientX, clientY, index) {
    activeIndex = index;
    menuEl.hidden = false;

    const menuWidth = menuEl.offsetWidth || 160;
    const menuHeight = menuEl.offsetHeight || 120;
    const maxX = window.innerWidth - menuWidth - 8;
    const maxY = window.innerHeight - menuHeight - 8;
    menuEl.style.left = `${Math.max(8, Math.min(clientX, maxX))}px`;
    menuEl.style.top = `${Math.max(8, Math.min(clientY, maxY))}px`;
  }

  menuEl.querySelectorAll("[data-action]").forEach((button) => {
    button.addEventListener("click", () => {
      const action = button.dataset.action;
      const index = activeIndex;
      hide();
      if (index == null || !handlers[action]) {
        return;
      }
      handlers[action](index);
    });
  });

  document.addEventListener("click", (event) => {
    if (!menuEl.hidden && !menuEl.contains(event.target)) {
      hide();
    }
  });
  document.addEventListener("contextmenu", (event) => {
    if (event.defaultPrevented) {
      return;
    }
    if (!menuEl.hidden && !menuEl.contains(event.target)) {
      hide();
    }
  });
  window.addEventListener("blur", hide);

  return { show, hide };
}

export function renderFileInfoBody(container, details) {
  const rows = [
    ["Name", details.name],
    ["Type", details.type],
    ["MIME", details.mimeType || "-"],
    ["Size", details.size || "-"],
    ["Date", details.date || "-"],
    ["File ID", details.fileID ? String(details.fileID) : "-"],
    ["On disk", details.onDisk ? "Yes" : "No"],
    ["Disk path", details.diskPath || "-"],
    ["Bunkr page", details.fileURL],
    ["Thumbnail", details.previewURL],
    ["Media URL", details.mediaURL || details.mediaURLError || "-"],
  ];

  container.replaceChildren();
  for (const [label, value] of rows) {
    const row = document.createElement("div");
    row.className = "info-row";

    const key = document.createElement("span");
    key.className = "info-label";
    key.textContent = label;

    const val = document.createElement("span");
    val.className = "info-value";
    val.textContent = value || "-";

    row.append(key, val);
    container.append(row);
  }
}
