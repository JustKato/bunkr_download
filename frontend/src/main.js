import { Events } from "/wails/runtime.js";
import {
  CancelDownload,
  ChooseOutputFolder,
  DownloadFileAtIndex,
  GetDownloadProgress,
  GetDownloadedFileIndices,
  GetFileDetails,
  GetAlbumHistory,
  GetSettings,
  OpenOutputFolder,
  OpenPreview,
  Quit,
  SaveSettings,
  ScrapeAlbum,
  SetOutputFolder,
  StartDownload,
} from "../bindings/github.com/justkato/bunkr_download/bunkrservice.js";
import { initContextMenu, renderFileInfoBody } from "./context-menu.js";
import { initMenu } from "./menu.js";

const input = document.getElementById("url-input");
const historySelect = document.getElementById("album-history");
const goBtn = document.getElementById("go-btn");
const statusText = document.getElementById("status-text");
const panelTitle = document.getElementById("panel-title");
const albumSummary = document.getElementById("album-summary");
const albumName = document.getElementById("album-name");
const albumSource = document.getElementById("album-source");
const albumStats = document.getElementById("album-stats");
const fileList = document.getElementById("file-list");
const sidebar = document.getElementById("sidebar");
const outputFolderInput = document.getElementById("output-folder");
const browseFolderBtn = document.getElementById("browse-folder-btn");
const downloadBtn = document.getElementById("download-btn");
const cancelDownloadBtn = document.getElementById("cancel-download-btn");
const filterImage = document.getElementById("filter-image");
const filterVideo = document.getElementById("filter-video");
const filterAudio = document.getElementById("filter-audio");
const filterFile = document.getElementById("filter-file");
const includePatterns = document.getElementById("include-patterns");
const currentFileLabel = document.getElementById("current-file-label");
const overallLabel = document.getElementById("overall-label");
const currentProgress = document.getElementById("current-progress");
const overallProgress = document.getElementById("overall-progress");
const aboutModal = document.getElementById("about-modal");
const aboutCloseBtn = document.getElementById("about-close-btn");
const statusbar = document.getElementById("statusbar");
const fileInfoModal = document.getElementById("file-info-modal");
const fileInfoBody = document.getElementById("file-info-body");
const fileInfoCloseBtn = document.getElementById("file-info-close-btn");
const fileContextMenu = document.getElementById("file-context-menu");

let currentAlbum = null;
let downloadRunning = false;
const rowStatus = new Map();
let saveSettingsTimer = null;

function getEmptyState() {
  return document.getElementById("empty-state");
}

function ensureEmptyState() {
  let state = getEmptyState();
  if (state) {
    return state;
  }

  const panelBody = document.querySelector(".main-panel .panel-body");
  state = document.createElement("div");
  state.id = "empty-state";
  state.className = "empty-state";
  panelBody.prepend(state);
  return state;
}

function showEmptyStateDefault() {
  albumSummary.hidden = true;
  fileList.replaceChildren();
  const state = ensureEmptyState();
  state.className = "empty-state";
  state.hidden = false;
  state.innerHTML = `
    <p class="empty-title">NO ARCHIVE LOADED</p>
    <p class="empty-hint">Enter a Bunkr album URL above and press FETCH.</p>
    <p class="empty-note">Click a file to preview it.</p>
  `;
}

function setEmptyStateLoading(message, hint = "Large albums can take a little while.") {
  albumSummary.hidden = true;
  fileList.replaceChildren();
  const state = ensureEmptyState();
  state.className = "empty-state empty-state-loading";
  state.hidden = false;
  state.innerHTML = `
    <p class="empty-title">${message}</p>
    <div class="empty-spinner" aria-hidden="true"></div>
    <p class="empty-hint">${hint}</p>
  `;
}

function hideEmptyState() {
  const state = getEmptyState();
  if (state) {
    state.remove();
  }
}

function setStatus(msg, isError = false) {
  statusText.textContent = msg;
  statusText.classList.toggle("error", isError);
  statusbar?.classList.toggle("error", isError);
}

function setLoading(isLoading) {
  goBtn.disabled = isLoading;
  goBtn.textContent = isLoading ? "LOADING..." : "FETCH";
  input.disabled = isLoading;
  if (historySelect) {
    historySelect.disabled = isLoading;
  }
}

function formatHistoryLabel(entry) {
  const id = entry.id || "unknown";
  const title = entry.title || "Untitled";
  return `${id} | ${title}`;
}

function renderAlbumHistory(history) {
  if (!historySelect) {
    return;
  }

  historySelect.replaceChildren();
  const placeholder = document.createElement("option");
  placeholder.value = "";
  placeholder.textContent = "HISTORY";
  historySelect.append(placeholder);

  for (const entry of history) {
    const option = document.createElement("option");
    option.value = entry.url || "";
    option.textContent = formatHistoryLabel(entry);
    historySelect.append(option);
  }
}

async function refreshAlbumHistory() {
  try {
    const history = await GetAlbumHistory();
    renderAlbumHistory(history || []);
  } catch {
    renderAlbumHistory([]);
  }
}

function eventData(event) {
  if (event && event.data != null) {
    return event.data;
  }
  return event;
}

function canPreview(file) {
  const type = String(file.type || "").toLowerCase();
  if (type === "image" || type === "video") return true;
  const mime = String(file.mimeType || "").toLowerCase();
  if (mime.startsWith("image/") || mime.startsWith("video/")) return true;
  return String(file.name || "").toLowerCase().endsWith(".pdf");
}

function typeIcon(type) {
  switch (String(type || "File").toLowerCase()) {
    case "image":
      return "IMG";
    case "video":
      return "VID";
    case "audio":
      return "AUD";
    default:
      return "FILE";
  }
}

function getFilterTypes() {
  const types = [];
  if (filterImage.checked) types.push("Image");
  if (filterVideo.checked) types.push("Video");
  if (filterAudio.checked) types.push("Audio");
  if (filterFile.checked) types.push("File");
  return types;
}

function getDownloadOptions() {
  const patterns = includePatterns.value
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);

  return { types: getFilterTypes(), includePatterns: patterns };
}

function buildSettingsFromUI() {
  return {
    outputFolder: outputFolderInput.value.trim(),
    filterTypes: getFilterTypes(),
    includePatterns: includePatterns.value.trim(),
  };
}

function applySettingsToUI(settings) {
  outputFolderInput.value = settings.outputFolder || "";
  const types = new Set(settings.filterTypes || []);
  filterImage.checked = types.size === 0 || types.has("Image");
  filterVideo.checked = types.size === 0 || types.has("Video");
  filterAudio.checked = types.size === 0 || types.has("Audio");
  filterFile.checked = types.size === 0 || types.has("File");
  includePatterns.value = settings.includePatterns || "";
}

function scheduleSaveSettings() {
  if (saveSettingsTimer) {
    clearTimeout(saveSettingsTimer);
  }
  saveSettingsTimer = setTimeout(() => {
    saveSettingsTimer = null;
    persistSettings().catch((error) => {
      const message = error instanceof Error ? error.message : String(error);
      setStatus(`Settings save failed: ${message}`, true);
    });
  }, 300);
}

async function persistSettings() {
  await SaveSettings(buildSettingsFromUI());
}

function resetFilters() {
  filterImage.checked = true;
  filterVideo.checked = true;
  filterAudio.checked = true;
  filterFile.checked = true;
  includePatterns.value = "";
  scheduleSaveSettings();
}

function updateDownloadControls() {
  downloadBtn.disabled = !currentAlbum || downloadRunning;
  cancelDownloadBtn.disabled = !downloadRunning;
}

function resetProgressUI(total = 0) {
  currentFileLabel.textContent = "CURRENT: -";
  overallLabel.textContent = `OVERALL: 0 / ${total}`;
  currentProgress.style.width = "0%";
  overallProgress.style.width = "0%";
}

async function loadSettings() {
  try {
    const settings = await GetSettings();
    applySettingsToUI(settings);
  } catch {
    outputFolderInput.value = "";
  }
}

async function saveOutputFolderFromInput() {
  const path = outputFolderInput.value.trim();
  try {
    await SetOutputFolder(path);
    scheduleSaveSettings();
    if (path) {
      setStatus(`Output folder: ${path}`);
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(`Folder save failed: ${message}`, true);
  }
}

async function chooseOutputFolder() {
  try {
    const path = await ChooseOutputFolder();
    if (path) {
      outputFolderInput.value = path;
      setStatus(`Output folder: ${path}`);
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(`Folder picker failed: ${message}`, true);
  }
}

async function startDownload() {
  if (!currentAlbum) {
    setStatus("Load an album first", true);
    return;
  }

  resetProgressUI(0);
  rowStatus.clear();
  fileList.querySelectorAll(".file-status").forEach((badge) => {
    badge.hidden = true;
  });

  try {
    await StartDownload(getDownloadOptions());
    downloadRunning = true;
    updateDownloadControls();
    setStatus("Download started");

    try {
      const snapshot = await GetDownloadProgress();
      if (snapshot) {
        handleDownloadProgress(snapshot);
      }
    } catch {}
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(`Download failed: ${message}`, true);
  }
}

async function cancelDownload() {
  try {
    await CancelDownload();
    setStatus("Cancel requested");
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(`Cancel failed: ${message}`, true);
  }
}

function updateProgressUI(progress) {
  const completed = progress.completedCount || 0;
  const total = progress.totalCount || 0;
  const currentTotal = progress.currentTotal || 0;
  const currentBytes = progress.currentBytes || 0;

  currentFileLabel.textContent = progress.currentName
    ? `CURRENT: ${progress.currentName}`
    : "CURRENT: -";

  overallLabel.textContent = `OVERALL: ${completed} / ${total}`;

  const currentPct =
    currentTotal > 0 ? Math.min(100, (currentBytes / currentTotal) * 100) : 0;
  currentProgress.style.width = `${currentPct}%`;

  const overallFraction =
    total > 0 ? (completed + (currentTotal > 0 ? currentBytes / currentTotal : 0)) / total : 0;
  overallProgress.style.width = `${Math.min(100, overallFraction * 100)}%`;

  if (progress.currentIndex >= 0 && progress.fileStatus) {
    const normalized = progress.fileStatus.toUpperCase();
    if (normalized === "SKIPPED" || normalized === "DONE") {
      rowStatus.set(progress.currentIndex, "ON DISK");
    } else {
      rowStatus.set(progress.currentIndex, normalized);
    }
    updateRowBadge(progress.currentIndex);
  }
}

function updateRowBadge(index) {
  const row = fileList.querySelector(`[data-index="${index}"]`);
  if (!row) return;
  const badge = row.querySelector(".file-status");
  const status = rowStatus.get(index);
  if (!badge || !status) return;
  badge.textContent = status;
  badge.hidden = false;
  badge.classList.toggle("on-disk", status === "ON DISK");
  badge.classList.toggle("failed", status === "FAILED" || status === "CANCELLED");
}

function handleDownloadProgress(progress) {
  if (!progress) return;

  downloadRunning = !!progress.running;
  updateDownloadControls();
  updateProgressUI(progress);

  if (progress.error) {
    if (progress.currentIndex >= 0) {
      rowStatus.set(progress.currentIndex, progress.cancelled ? "CANCELLED" : "FAILED");
      updateRowBadge(progress.currentIndex);
    }
    setStatus(`Download failed: ${progress.error}`, true);
    return;
  }

  if (progress.running) {
    if (progress.currentName) {
      setStatus(`Downloading ${progress.currentName}`);
    }
    return;
  }

  if (progress.cancelled) {
    setStatus("Download cancelled", true);
    return;
  }

  setStatus(`Done: ${progress.completedCount}/${progress.totalCount}`);
  markDownloadedFiles().catch(() => {});
}

async function markDownloadedFiles() {
  if (!currentAlbum) return;
  try {
    const indices = await GetDownloadedFileIndices();
    for (const index of indices) {
      rowStatus.set(index, "ON DISK");
      updateRowBadge(index);
    }
  } catch {}
}

async function openFilePreview(index) {
  try {
    await OpenPreview(index);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(`Preview failed: ${message}`, true);
  }
}

async function downloadSingleFile(index) {
  if (downloadRunning) {
    setStatus("Another download is already running", true);
    return;
  }

  resetProgressUI(1);
  try {
    await DownloadFileAtIndex(index);
    downloadRunning = true;
    updateDownloadControls();
    setStatus("Download started");

    try {
      const snapshot = await GetDownloadProgress();
      if (snapshot) {
        handleDownloadProgress(snapshot);
      }
    } catch {}
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(`Download failed: ${message}`, true);
  }
}

async function showFileAbout(index) {
  try {
    const details = await GetFileDetails(index);
    renderFileInfoBody(fileInfoBody, details);
    fileInfoModal.hidden = false;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(`File info failed: ${message}`, true);
  }
}

function hideFileInfo() {
  fileInfoModal.hidden = true;
}

function makeFileRow(file, index) {
  const previewable = canPreview(file);
  const row = document.createElement("article");
  row.className = previewable ? "file-row file-row-previewable" : "file-row";
  row.dataset.index = String(index);

  const preview = document.createElement("div");
  preview.className = "file-preview";

  const previewFallback = document.createElement("span");
  previewFallback.className = "file-preview-fallback";
  previewFallback.textContent = typeIcon(file.type);
  preview.append(previewFallback);

  if (file.previewURL) {
    const image = document.createElement("img");
    image.src = file.previewURL;
    image.alt = "";
    image.loading = "lazy";
    image.addEventListener("load", () => preview.classList.add("has-preview"));
    image.addEventListener("error", () => image.remove());
    preview.append(image);
  }

  const details = document.createElement("div");
  details.className = "file-details";

  const nameRow = document.createElement("div");
  nameRow.className = "file-name-row";

  const name = document.createElement("a");
  name.className = "file-name";
  name.href = file.fileURL;
  name.target = "_blank";
  name.rel = "noopener noreferrer";
  name.textContent = file.name || "Unnamed file";
  name.addEventListener("click", (event) => event.stopPropagation());

  const statusBadge = document.createElement("span");
  statusBadge.className = "file-status";
  statusBadge.hidden = true;

  nameRow.append(name, statusBadge);

  const meta = document.createElement("div");
  meta.className = "file-meta";

  const type = document.createElement("span");
  type.className = "file-type";
  type.textContent = (file.type || "File").toUpperCase();

  const size = document.createElement("span");
  size.textContent = file.size || "SIZE UNKNOWN";

  const date = document.createElement("span");
  date.textContent = file.date || "DATE UNKNOWN";

  meta.append(type, size, date);
  details.append(nameRow, meta);
  row.append(preview, details);

  if (previewable) {
    row.addEventListener("click", () => openFilePreview(index));
  } else {
    row.addEventListener("click", () => {
      setStatus("No preview for this file", true);
    });
  }

  row.addEventListener("contextmenu", (event) => {
    event.preventDefault();
    fileMenu?.show(event.clientX, event.clientY, index);
  });

  return row;
}

async function renderAlbum(album) {
  currentAlbum = album;
  rowStatus.clear();
  hideEmptyState();
  albumSummary.hidden = false;
  fileList.replaceChildren();

  panelTitle.textContent = `ALBUM FILES (${album.files.length})`;
  albumName.textContent = album.title;
  albumSource.textContent = album.url;
  albumStats.textContent = `${album.files.length} FILES  /  ${album.totalSize || "SIZE UNKNOWN"}`;
  updateDownloadControls();

  const batchSize = 40;
  for (let i = 0; i < album.files.length; i += batchSize) {
    const fragment = document.createDocumentFragment();
    const end = Math.min(i + batchSize, album.files.length);
    for (let j = i; j < end; j++) {
      fragment.append(makeFileRow(album.files[j], j));
    }
    fileList.append(fragment);
    if (end < album.files.length) {
      setStatus(`Rendering files... ${end}/${album.files.length}`);
      await new Promise((resolve) => setTimeout(resolve, 0));
    }
  }

  markDownloadedFiles().catch(() => {});
}

function resetAlbumView() {
  currentAlbum = null;
  rowStatus.clear();
  panelTitle.textContent = "ALBUM FILES";
  showEmptyStateDefault();
  updateDownloadControls();
}

async function onFetch() {
  const raw = input.value.trim();
  if (!raw) {
    setStatus("Enter a URL first", true);
    return;
  }

  setLoading(true);
  setEmptyStateLoading("FETCHING ALBUM...");
  setStatus("Scraping album...");

  try {
    const album = await ScrapeAlbum(raw);
    if (!album?.files?.length) {
      throw new Error("Album returned no files");
    }

    setEmptyStateLoading(
      `RENDERING ${album.files.length} FILES...`,
      "Hang tight. Big albums render in batches so the UI stays responsive.",
    );
    setStatus(`Rendering ${album.files.length} files...`);
    await renderAlbum(album);
    await refreshAlbumHistory();
    setStatus(`Loaded ${album.files.length} files`);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    showEmptyStateDefault();
    setStatus(`Scrape failed: ${message}`, true);
  } finally {
    setLoading(false);
    input.focus();
  }
}

async function newAlbum() {
  if (downloadRunning) {
    await cancelDownload();
  }
  input.value = "";
  resetAlbumView();
  setStatus("Ready");
  input.focus();
}

function showAbout() {
  aboutModal.hidden = false;
}

function hideAbout() {
  aboutModal.hidden = true;
}

initMenu({
  "new-album": () => newAlbum(),
  "choose-folder": () => chooseOutputFolder(),
  exit: () => Quit(),
  "download-album": () => startDownload(),
  "cancel-download": () => cancelDownload(),
  "open-folder": () =>
    OpenOutputFolder().catch((error) => {
      const message = error instanceof Error ? error.message : String(error);
      setStatus(`Open folder failed: ${message}`, true);
    }),
  "focus-sidebar": () => sidebar.focus({ preventScroll: false }),
  "reset-filters": () => resetFilters(),
  about: () => showAbout(),
});

goBtn.addEventListener("click", onFetch);
input.addEventListener("keydown", (e) => {
  if (e.key === "Enter") onFetch();
});
if (historySelect) {
  historySelect.addEventListener("change", () => {
    const url = historySelect.value.trim();
    historySelect.value = "";
    if (!url) {
      return;
    }
    input.value = url;
    onFetch();
  });
}

browseFolderBtn.addEventListener("click", () => chooseOutputFolder());
downloadBtn.addEventListener("click", () => startDownload());
cancelDownloadBtn.addEventListener("click", () => cancelDownload());
aboutCloseBtn.addEventListener("click", () => hideAbout());
aboutModal.addEventListener("click", (event) => {
  if (event.target === aboutModal) hideAbout();
});
fileInfoCloseBtn.addEventListener("click", () => hideFileInfo());
fileInfoModal.addEventListener("click", (event) => {
  if (event.target === fileInfoModal) hideFileInfo();
});

const fileMenu = fileContextMenu
  ? initContextMenu(fileContextMenu, {
      open: (index) => openFilePreview(index),
      download: (index) => downloadSingleFile(index),
      about: (index) => showFileAbout(index),
    })
  : null;

outputFolderInput.addEventListener("change", () => saveOutputFolderFromInput());
outputFolderInput.addEventListener("blur", () => saveOutputFolderFromInput());
outputFolderInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    event.preventDefault();
    saveOutputFolderFromInput();
    outputFolderInput.blur();
  }
});

for (const checkbox of [filterImage, filterVideo, filterAudio, filterFile]) {
  checkbox.addEventListener("change", () => scheduleSaveSettings());
}

includePatterns.addEventListener("change", () => scheduleSaveSettings());
includePatterns.addEventListener("blur", () => scheduleSaveSettings());
includePatterns.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    event.preventDefault();
    scheduleSaveSettings();
    includePatterns.blur();
  }
});

Events.On("download:progress", (event) => {
  handleDownloadProgress(eventData(event));
});

loadSettings();
refreshAlbumHistory();
updateDownloadControls();
input.focus();
