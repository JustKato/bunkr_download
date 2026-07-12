import { Events } from "/wails/runtime.js";
import {
  CancelDownload,
  ChooseOutputFolder,
  DownloadFileAtIndex,
  GetDownloadProgress,
  GetDownloadedFileIndices,
  GetAlbumHistory,
  GetSettings,
  OpenAbout,
  OpenConsole,
  OpenFileInfo,
  OpenOutputFolder,
  OpenPreview,
  Quit,
  SaveSettings,
  ScrapeAlbum,
  SetOutputFolder,
  StartDownload,
} from "../bindings/github.com/justkato/bunkr_download/bunkrservice.js";
import { DownloadOptions } from "../bindings/github.com/justkato/bunkr_download/models.js";
import { initContextMenu } from "./context-menu.js";
import { initHistoryPicker } from "./history-picker.js";
import { installLogger, log } from "./logger.js";
import { initMenu } from "./menu.js";

installLogger();

const input = document.getElementById("url-input");
const historyPickerRoot = document.getElementById("history-picker");
const historyTrigger = document.getElementById("history-trigger");
const historyMenu = document.getElementById("history-menu");
const fileMenuHistory = document.getElementById("file-menu-history");
const goBtn = document.getElementById("go-btn");
const statusText = document.getElementById("status-text");
const panelTitle = document.getElementById("panel-title");
const albumSummary = document.getElementById("album-summary");
const albumName = document.getElementById("album-name");
const albumSource = document.getElementById("album-source");
const albumStats = document.getElementById("album-stats");
const fileList = document.getElementById("file-list");
const fileViewShell = document.getElementById("file-view-shell");
const paginationBar = document.getElementById("pagination-bar");
const pagePrevBtn = document.getElementById("page-prev");
const pageNextBtn = document.getElementById("page-next");
const pageInfo = document.getElementById("page-info");
const infiniteSentinel = document.getElementById("infinite-sentinel");
const panelBody = document.querySelector(".main-panel .panel-body");
const sidebar = document.getElementById("sidebar");
const outputFolderInput = document.getElementById("output-folder");
const browseFolderBtn = document.getElementById("browse-folder-btn");
const downloadBtn = document.getElementById("download-btn");
const cancelDownloadBtn = document.getElementById("cancel-download-btn");
const consoleBtn = document.getElementById("console-btn");
const filterImage = document.getElementById("filter-image");
const filterVideo = document.getElementById("filter-video");
const filterAudio = document.getElementById("filter-audio");
const filterFile = document.getElementById("filter-file");
const includePatterns = document.getElementById("include-patterns");
const currentFileLabel = document.getElementById("current-file-label");
const overallLabel = document.getElementById("overall-label");
const currentProgress = document.getElementById("current-progress");
const overallProgress = document.getElementById("overall-progress");
const statusbar = document.getElementById("statusbar");
const fileContextMenu = document.getElementById("file-context-menu");

let currentAlbum = null;
let downloadRunning = false;
let progressPollTimer = null;
const rowStatus = new Map();
let saveSettingsTimer = null;
let currentPage = 1;
let infiniteRenderedCount = 0;
let infiniteObserver = null;

const PAGE_SIZE = 25;

const viewSettings = {
  paginationMode: "pagination",
  viewMode: "list",
};

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
  if (fileViewShell) {
    fileViewShell.hidden = true;
  }
  fileList.replaceChildren();
  teardownInfiniteScroll();
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
  if (fileViewShell) {
    fileViewShell.hidden = true;
  }
  fileList.replaceChildren();
  teardownInfiniteScroll();
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
  historyPicker?.setDisabled(isLoading);
}

function formatHistoryLabel(entry) {
  const id = entry.id || "unknown";
  const title = entry.title || "Untitled";
  return `${id} | ${title}`;
}

function selectHistoryUrl(url) {
  const trimmed = String(url || "").trim();
  if (!trimmed) {
    return;
  }
  input.value = trimmed;
  onFetch();
}

function renderFileMenuHistory(history) {
  if (!fileMenuHistory) {
    return;
  }

  fileMenuHistory.replaceChildren();
  if (!history.length) {
    const empty = document.createElement("p");
    empty.className = "menu-history-empty";
    empty.textContent = "No albums yet";
    fileMenuHistory.append(empty);
    return;
  }

  for (const entry of history) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "menu-history-item";
    button.textContent = formatHistoryLabel(entry);
    button.title = entry.url || "";
    button.addEventListener("click", (event) => {
      event.stopPropagation();
      menuControls?.closeMenus();
      selectHistoryUrl(entry.url);
    });
    fileMenuHistory.append(button);
  }
}

function renderAlbumHistory(history) {
  historyPicker?.render(history, formatHistoryLabel);
  renderFileMenuHistory(history);
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
    paginationMode: viewSettings.paginationMode,
    viewMode: viewSettings.viewMode,
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
  viewSettings.paginationMode =
    settings.paginationMode === "infinite-scroll" ? "infinite-scroll" : "pagination";
  viewSettings.viewMode = settings.viewMode === "gallery" ? "gallery" : "list";
  applyViewModeClasses();
  updateViewMenuChecks();
}

function applyViewModeClasses() {
  fileList.classList.toggle("file-list-mode-list", viewSettings.viewMode === "list");
  fileList.classList.toggle("file-list-mode-gallery", viewSettings.viewMode === "gallery");
}

function updateViewMenuChecks() {
  for (const button of document.querySelectorAll("[data-check-group]")) {
    const group = button.dataset.checkGroup;
    const value = button.dataset.checkValue;
    const current =
      group === "paginationMode" ? viewSettings.paginationMode : viewSettings.viewMode;
    const checked = value === current;
    button.classList.toggle("checked", checked);
    button.setAttribute("aria-checked", checked ? "true" : "false");
  }
}

function setPaginationMode(mode) {
  const nextMode = mode === "infinite-scroll" ? "infinite-scroll" : "pagination";
  if (viewSettings.paginationMode === nextMode) {
    return;
  }
  viewSettings.paginationMode = nextMode;
  currentPage = 1;
  infiniteRenderedCount = 0;
  updateViewMenuChecks();
  scheduleSaveSettings();
  if (currentAlbum) {
    renderVisibleFiles();
  }
}

function setViewMode(mode) {
  const nextMode = mode === "gallery" ? "gallery" : "list";
  if (viewSettings.viewMode === nextMode) {
    return;
  }
  viewSettings.viewMode = nextMode;
  applyViewModeClasses();
  updateViewMenuChecks();
  scheduleSaveSettings();
  if (currentAlbum) {
    renderVisibleFiles();
  }
}

function getTotalPages(totalFiles = currentAlbum?.files?.length || 0) {
  return Math.max(1, Math.ceil(totalFiles / PAGE_SIZE));
}

function teardownInfiniteScroll() {
  if (infiniteObserver) {
    infiniteObserver.disconnect();
    infiniteObserver = null;
  }
  if (infiniteSentinel) {
    infiniteSentinel.hidden = true;
  }
}

function setupInfiniteScroll() {
  teardownInfiniteScroll();
  if (!currentAlbum || viewSettings.paginationMode !== "infinite-scroll" || !infiniteSentinel) {
    return;
  }

  infiniteSentinel.hidden = false;
  infiniteObserver = new IntersectionObserver(
    (entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        loadMoreInfiniteItems();
      }
    },
    {
      root: panelBody,
      rootMargin: "240px",
    },
  );
  infiniteObserver.observe(infiniteSentinel);
}

function updatePaginationControls() {
  if (!paginationBar || !currentAlbum || viewSettings.paginationMode !== "pagination") {
    if (paginationBar) {
      paginationBar.hidden = true;
    }
    return;
  }

  const total = currentAlbum.files.length;
  const totalPages = getTotalPages(total);
  currentPage = Math.min(Math.max(currentPage, 1), totalPages);

  paginationBar.hidden = total <= PAGE_SIZE;
  if (pageInfo) {
    pageInfo.textContent = `Page ${currentPage} / ${totalPages}`;
  }
  if (pagePrevBtn) {
    pagePrevBtn.disabled = currentPage <= 1;
  }
  if (pageNextBtn) {
    pageNextBtn.disabled = currentPage >= totalPages;
  }
}

function appendFileItems(start, end) {
  if (!currentAlbum) {
    return;
  }

  const fragment = document.createDocumentFragment();
  for (let index = start; index < end; index++) {
    fragment.append(makeFileItem(currentAlbum.files[index], index));
  }

  if (viewSettings.paginationMode === "infinite-scroll" && infiniteSentinel) {
    fileList.insertBefore(fragment, infiniteSentinel);
  } else {
    fileList.append(fragment);
  }

  for (let index = start; index < end; index++) {
    updateRowBadge(index);
  }
}

function loadMoreInfiniteItems() {
  if (!currentAlbum || viewSettings.paginationMode !== "infinite-scroll") {
    return;
  }

  const total = currentAlbum.files.length;
  if (infiniteRenderedCount >= total) {
    if (infiniteSentinel) {
      infiniteSentinel.hidden = true;
    }
    return;
  }

  const start = infiniteRenderedCount;
  const end = Math.min(start + PAGE_SIZE, total);
  appendFileItems(start, end);
  infiniteRenderedCount = end;

  if (infiniteRenderedCount >= total && infiniteSentinel) {
    infiniteSentinel.hidden = true;
  }
}

function renderVisibleFiles() {
  if (!currentAlbum) {
    return;
  }

  teardownInfiniteScroll();
  fileList.replaceChildren();
  infiniteRenderedCount = 0;
  currentPage = Math.min(currentPage, getTotalPages(currentAlbum.files.length));

  if (viewSettings.paginationMode === "infinite-scroll") {
    if (paginationBar) {
      paginationBar.hidden = true;
    }
    if (infiniteSentinel) {
      fileList.append(infiniteSentinel);
    }
    loadMoreInfiniteItems();
    setupInfiniteScroll();
    return;
  }

  const start = (currentPage - 1) * PAGE_SIZE;
  const end = Math.min(start + PAGE_SIZE, currentAlbum.files.length);
  appendFileItems(start, end);
  updatePaginationControls();
  panelBody?.scrollTo({ top: 0, behavior: "auto" });
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

function beginProgressPolling() {
  stopProgressPolling();
  progressPollTimer = setInterval(async () => {
    try {
      const snapshot = await GetDownloadProgress();
      if (snapshot) {
        handleDownloadProgress(snapshot);
      }
    } catch (error) {
      log.warn("download", error instanceof Error ? error.message : String(error));
    }
  }, 500);
}

function stopProgressPolling() {
  if (progressPollTimer) {
    clearInterval(progressPollTimer);
    progressPollTimer = null;
  }
}

async function startDownload() {
  if (!currentAlbum) {
    setStatus("Load an album first", true);
    return;
  }

  log.info("download", "Download Album clicked");
  resetProgressUI(0);
  rowStatus.clear();
  fileList.querySelectorAll(".file-status").forEach((badge) => {
    badge.hidden = true;
  });

  try {
    await StartDownload(new DownloadOptions(getDownloadOptions()));
    downloadRunning = true;
    updateDownloadControls();
    setStatus("Download started");
    beginProgressPolling();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    log.error("download", message);
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
    log.error("download", progress.error);
    setStatus(`Download failed: ${progress.error}`, true);
    stopProgressPolling();
    return;
  }

  if (progress.running) {
    if (progress.currentName) {
      const phase = progress.fileStatus ? ` (${progress.fileStatus})` : "";
      setStatus(`Downloading ${progress.currentName}${phase}`);
    }
    return;
  }

  stopProgressPolling();

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

  log.info("download", `single file download requested for index ${index}`);
  resetProgressUI(1);
  try {
    await DownloadFileAtIndex(index);
    downloadRunning = true;
    updateDownloadControls();
    setStatus("Download started");
    beginProgressPolling();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    log.error("download", message);
    setStatus(`Download failed: ${message}`, true);
  }
}

async function showFileAbout(index) {
  try {
    await OpenFileInfo(index);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(`File info failed: ${message}`, true);
  }
}

function activateFileItem(file, index) {
  if (canPreview(file)) {
    openFilePreview(index);
  } else {
    setStatus("No preview for this file", true);
  }
}

function attachFileItemHandlers(item, file, index) {
  item.addEventListener("click", (event) => {
    if (event.defaultPrevented) {
      return;
    }
    activateFileItem(file, index);
  });

  item.addEventListener("contextmenu", (event) => {
    event.preventDefault();
    fileMenu?.show(event.clientX, event.clientY, index);
  });
}

function appendPreviewMedia(container, file, loadedClass) {
  const previewFallback = document.createElement("span");
  previewFallback.className = container.classList.contains("gallery-thumb")
    ? "gallery-thumb-fallback"
    : "file-preview-fallback";
  previewFallback.textContent = typeIcon(file.type);
  container.append(previewFallback);

  if (file.previewURL) {
    const image = document.createElement("img");
    image.src = file.previewURL;
    image.alt = "";
    image.loading = "lazy";
    image.addEventListener("load", () => container.classList.add(loadedClass));
    image.addEventListener("error", () => image.remove());
    container.append(image);
  }
}

function makeStatusBadge() {
  const statusBadge = document.createElement("span");
  statusBadge.className = "file-status";
  statusBadge.hidden = true;
  return statusBadge;
}

function makeFileRow(file, index) {
  const previewable = canPreview(file);
  const row = document.createElement("article");
  row.className = previewable ? "file-row file-row-previewable" : "file-row";
  row.dataset.index = String(index);

  const preview = document.createElement("div");
  preview.className = "file-preview";
  appendPreviewMedia(preview, file, "has-preview");

  const details = document.createElement("div");
  details.className = "file-details";

  const nameRow = document.createElement("div");
  nameRow.className = "file-name-row";

  const name = document.createElement("span");
  name.className = "file-name";
  name.textContent = file.name || "Unnamed file";
  name.title = file.name || "Unnamed file";

  const statusBadge = makeStatusBadge();
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
  attachFileItemHandlers(row, file, index);
  return row;
}

function makeFileGalleryCard(file, index) {
  const previewable = canPreview(file);
  const card = document.createElement("article");
  card.className = previewable ? "gallery-card gallery-card-previewable" : "gallery-card";
  card.dataset.index = String(index);

  const thumb = document.createElement("div");
  thumb.className = "gallery-thumb";
  appendPreviewMedia(thumb, file, "has-preview");

  const statusBadge = makeStatusBadge();
  thumb.append(statusBadge);

  const caption = document.createElement("div");
  caption.className = "gallery-caption";

  const name = document.createElement("p");
  name.className = "gallery-name";
  name.textContent = file.name || "Unnamed file";
  name.title = file.name || "Unnamed file";

  const meta = document.createElement("p");
  meta.className = "gallery-meta";
  meta.textContent = `${(file.type || "File").toUpperCase()} · ${file.size || "SIZE UNKNOWN"}`;

  caption.append(name, meta);
  card.append(thumb, caption);
  attachFileItemHandlers(card, file, index);
  return card;
}

function makeFileItem(file, index) {
  if (viewSettings.viewMode === "gallery") {
    return makeFileGalleryCard(file, index);
  }
  return makeFileRow(file, index);
}

async function renderAlbum(album) {
  currentAlbum = album;
  rowStatus.clear();
  currentPage = 1;
  infiniteRenderedCount = 0;
  hideEmptyState();
  albumSummary.hidden = false;
  if (fileViewShell) {
    fileViewShell.hidden = false;
  }
  applyViewModeClasses();

  panelTitle.textContent = `ALBUM FILES (${album.files.length})`;
  albumName.textContent = album.title;
  albumSource.textContent = album.url;
  albumStats.textContent = `${album.files.length} FILES  /  ${album.totalSize || "SIZE UNKNOWN"}`;
  updateDownloadControls();

  renderVisibleFiles();
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

  log.info("fetch", `scraping album: ${raw}`);
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
    log.error("fetch", message);
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
  OpenAbout().catch((error) => {
    const message = error instanceof Error ? error.message : String(error);
    setStatus(`About window failed: ${message}`, true);
  });
}

async function showConsole() {
  log.info("console", "console open requested");
  try {
    setStatus("Opening console...");
    await OpenConsole();
    setStatus("Console opened");
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    log.error("console", message);
    setStatus(`Console failed: ${message}`, true);
  }
}

const historyPicker =
  historyPickerRoot && historyTrigger && historyMenu
    ? initHistoryPicker({
        root: historyPickerRoot,
        trigger: historyTrigger,
        menu: historyMenu,
        onSelect: selectHistoryUrl,
      })
    : null;

const menuControls = initMenu({
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
  "pagination-mode-pages": () => setPaginationMode("pagination"),
  "pagination-mode-infinite": () => setPaginationMode("infinite-scroll"),
  "view-mode-list": () => setViewMode("list"),
  "view-mode-gallery": () => setViewMode("gallery"),
  console: () => showConsole(),
  about: () => showAbout(),
});

goBtn.addEventListener("click", onFetch);
input.addEventListener("keydown", (e) => {
  if (e.key === "Enter") onFetch();
});

pagePrevBtn?.addEventListener("click", () => {
  if (currentPage > 1) {
    currentPage -= 1;
    renderVisibleFiles();
  }
});

pageNextBtn?.addEventListener("click", () => {
  if (currentAlbum && currentPage < getTotalPages()) {
    currentPage += 1;
    renderVisibleFiles();
  }
});

browseFolderBtn.addEventListener("click", () => chooseOutputFolder());
downloadBtn.addEventListener("click", () => startDownload());
cancelDownloadBtn.addEventListener("click", () => cancelDownload());
consoleBtn?.addEventListener("click", () => showConsole());

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
