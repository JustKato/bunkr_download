import {
  CacheMediaFile,
  DownloadFileAtIndex,
  GetActiveAlbum,
  GetFileDetails,
  GetPreviewIndex,
  ResolveMediaURL,
  SetPreviewIndex,
} from "../bindings/github.com/justkato/bunkr_download/bunkrservice.js";
import { initContextMenu, renderFileInfoBody } from "./context-menu.js";

const previewName = document.getElementById("preview-name");
const previewCounter = document.getElementById("preview-counter");
const previewMessage = document.getElementById("preview-message");
const previewStage = document.getElementById("preview-stage");
const previewViewport = document.getElementById("preview-viewport");
const previewMediaLayer = document.getElementById("preview-media-layer");
const previewMediaWrap = document.getElementById("preview-media-wrap");
const previewPrev = document.getElementById("preview-prev");
const previewNext = document.getElementById("preview-next");
const previewFlipX = document.getElementById("preview-flip-x");
const previewFlipY = document.getElementById("preview-flip-y");
const previewZoom = document.getElementById("preview-zoom");
const fileInfoModal = document.getElementById("file-info-modal");
const fileInfoBody = document.getElementById("file-info-body");
const fileInfoCloseBtn = document.getElementById("file-info-close-btn");
const fileContextMenu = document.getElementById("file-context-menu");

const state = {
  album: null,
  previewableIndices: [],
  currentListPos: 0,
  fitScale: 1,
  scale: 1,
  panX: 0,
  panY: 0,
  flipX: false,
  flipY: false,
  isImage: false,
  dragging: false,
  dragStartX: 0,
  dragStartY: 0,
  panStartX: 0,
  panStartY: 0,
  wheelNavTimer: null,
};

function canPreview(file) {
  const type = String(file.type || "").toLowerCase();
  if (type === "image" || type === "video") return true;
  const mime = String(file.mimeType || "").toLowerCase();
  if (mime.startsWith("image/") || mime.startsWith("video/")) return true;
  return String(file.name || "").toLowerCase().endsWith(".pdf");
}

function isPDF(file) {
  return (
    String(file.name || "").toLowerCase().endsWith(".pdf") ||
    String(file.mimeType || "").toLowerCase() === "application/pdf"
  );
}

function isVideo(file) {
  const type = String(file.type || "").toLowerCase();
  if (type === "video") return true;
  return String(file.mimeType || "").toLowerCase().startsWith("video/");
}

function isImage(file) {
  const type = String(file.type || "").toLowerCase();
  if (type === "image") return true;
  return String(file.mimeType || "").toLowerCase().startsWith("image/");
}

function buildPreviewableIndices(album) {
  return album.files
    .map((file, index) => (canPreview(file) ? index : -1))
    .filter((index) => index >= 0);
}

function listPosForAlbumIndex(albumIndex) {
  const pos = state.previewableIndices.indexOf(albumIndex);
  return pos >= 0 ? pos : 0;
}

function minScale() {
  return Math.max(state.fitScale * 0.25, 0.05);
}

function maxScale() {
  return Math.max(state.fitScale * 8, 4);
}

function clampScale(value) {
  return Math.min(maxScale(), Math.max(minScale(), value));
}

function currentAlbumIndex() {
  if (!state.album || state.previewableIndices.length === 0) {
    return -1;
  }
  return state.previewableIndices[state.currentListPos];
}

function showPreviewMessage(message, isError = false) {
  previewMessage.hidden = false;
  previewMessage.textContent = message;
  previewMessage.classList.toggle("error", isError);
}

async function showFileAbout(index) {
  try {
    const details = await GetFileDetails(index);
    renderFileInfoBody(fileInfoBody, details);
    fileInfoModal.hidden = false;
  } catch (error) {
    showPreviewMessage(
      error instanceof Error ? error.message : String(error),
      true,
    );
  }
}

async function downloadCurrentFile() {
  const index = currentAlbumIndex();
  if (index < 0) return;
  try {
    await DownloadFileAtIndex(index);
    showPreviewMessage("Download started");
  } catch (error) {
    showPreviewMessage(
      error instanceof Error ? error.message : String(error),
      true,
    );
  }
}

const previewFileMenu = initContextMenu(fileContextMenu, {
  open: () => renderCurrent().catch(console.error),
  download: () => downloadCurrentFile().catch(console.error),
  about: () => {
    const index = currentAlbumIndex();
    if (index >= 0) {
      showFileAbout(index).catch(console.error);
    }
  },
});

function bindPreviewContextMenu(target) {
  target.addEventListener("contextmenu", (event) => {
    const index = currentAlbumIndex();
    if (index < 0) return;
    event.preventDefault();
    previewFileMenu.show(event.clientX, event.clientY, index);
  });
}

bindPreviewContextMenu(previewStage);
bindPreviewContextMenu(previewViewport);
bindPreviewContextMenu(previewMediaWrap);

fileInfoCloseBtn?.addEventListener("click", () => {
  fileInfoModal.hidden = true;
});
fileInfoModal?.addEventListener("click", (event) => {
  if (event.target === fileInfoModal) {
    fileInfoModal.hidden = true;
  }
});

function updateToolbar() {
  previewPrev.disabled = state.currentListPos <= 0;
  previewNext.disabled = state.currentListPos >= state.previewableIndices.length - 1;
  previewFlipX.classList.toggle("active", state.flipX);
  previewFlipY.classList.toggle("active", state.flipY);
  const pct = state.fitScale > 0 ? Math.round((state.scale / state.fitScale) * 100) : 100;
  previewZoom.textContent = `${pct}%`;
  previewFlipX.disabled = !state.isImage;
  previewFlipY.disabled = !state.isImage;
}

function applyImageTransform() {
  if (!state.isImage) return;
  const scaleX = state.flipX ? -state.scale : state.scale;
  const scaleY = state.flipY ? -state.scale : state.scale;
  previewMediaLayer.style.transform = `translate(${state.panX}px, ${state.panY}px) scale(${scaleX}, ${scaleY})`;
}

function fitImageToViewport(img) {
  const viewportW = previewViewport.clientWidth;
  const viewportH = previewViewport.clientHeight;
  const naturalW = img.naturalWidth || 1;
  const naturalH = img.naturalHeight || 1;

  state.fitScale = Math.min(viewportW / naturalW, viewportH / naturalH, 1);
  state.scale = state.fitScale;
  state.panX = (viewportW - naturalW * state.scale) / 2;
  state.panY = (viewportH - naturalH * state.scale) / 2;

  img.style.width = `${naturalW}px`;
  img.style.height = `${naturalH}px`;
  img.style.maxWidth = "none";
  img.style.maxHeight = "none";

  applyImageTransform();
  updateToolbar();
}

function resetView() {
  state.flipX = false;
  state.flipY = false;
  state.scale = state.fitScale || 1;
  state.panX = 0;
  state.panY = 0;
  updateToolbar();
}

async function resolveMedia(file) {
  if (isPDF(file)) {
    if (file.fileID > 0) {
      return ResolveMediaURL(file.fileID);
    }
    return "";
  }

  if ((isImage(file) || isVideo(file)) && file.fileID > 0) {
    return ResolveMediaURL(file.fileID);
  }

  return "";
}

function clearMediaViews() {
  previewViewport.hidden = true;
  previewMediaWrap.hidden = true;
  previewMediaLayer.replaceChildren();
  previewMediaWrap.replaceChildren();
  state.isImage = false;
}

async function renderCurrent() {
  if (!state.album || state.previewableIndices.length === 0) {
    previewMessage.hidden = false;
    previewMessage.textContent = "Nothing to preview";
    clearMediaViews();
    return;
  }

  const albumIndex = state.previewableIndices[state.currentListPos];
  const file = state.album.files[albumIndex];
  previewName.textContent = file.name || "Unnamed file";
  previewCounter.textContent = `${state.currentListPos + 1} / ${state.previewableIndices.length}`;
  previewMessage.hidden = false;
  previewMessage.textContent = "Loading...";
  clearMediaViews();
  updateToolbar();

  let mediaURL = "";
  try {
    mediaURL = await resolveMedia(file);
  } catch (error) {
    previewMessage.textContent =
      error instanceof Error ? error.message : "Could not load image";
    return;
  }

  if (!mediaURL) {
    previewMessage.textContent = "Could not load image";
    return;
  }

  previewMessage.hidden = true;

  if (isPDF(file)) {
    previewMediaWrap.hidden = false;
    const iframe = document.createElement("iframe");
    iframe.src = mediaURL;
    iframe.title = file.name;
    previewMediaWrap.append(iframe);
    updateToolbar();
    return;
  }

  if (isVideo(file)) {
    previewMediaWrap.hidden = false;
    const video = document.createElement("video");
    video.src = mediaURL;
    video.controls = true;
    video.autoplay = true;
    previewMediaWrap.append(video);
    updateToolbar();
    return;
  }

  state.isImage = true;
  previewViewport.hidden = false;
  const img = document.createElement("img");
  img.src = mediaURL;
  img.alt = file.name || "";
  img.draggable = false;
  previewMediaLayer.append(img);

  img.addEventListener("load", () => {
    fitImageToViewport(img);
    if (file.fileID > 0) {
      CacheMediaFile(file.fileID).catch(() => {});
    }
  });

  if (img.complete && img.naturalWidth > 0) {
    fitImageToViewport(img);
    if (file.fileID > 0) {
      CacheMediaFile(file.fileID).catch(() => {});
    }
  }
}

async function goToAlbumIndex(albumIndex) {
  if (!state.album) return;
  state.currentListPos = listPosForAlbumIndex(albumIndex);
  resetView();
  await SetPreviewIndex(albumIndex);
  await renderCurrent();
}

async function step(delta) {
  const nextPos = state.currentListPos + delta;
  if (nextPos < 0 || nextPos >= state.previewableIndices.length) return;
  state.currentListPos = nextPos;
  resetView();
  await SetPreviewIndex(state.previewableIndices[nextPos]);
  await renderCurrent();
}

function zoomAtCursor(event, factor) {
  if (!state.isImage) return;

  const rect = previewViewport.getBoundingClientRect();
  const cursorX = event.clientX - rect.left;
  const cursorY = event.clientY - rect.top;

  const mx = (cursorX - state.panX) / state.scale;
  const my = (cursorY - state.panY) / state.scale;

  state.scale = clampScale(state.scale * factor);
  state.panX = cursorX - mx * state.scale;
  state.panY = cursorY - my * state.scale;

  applyImageTransform();
  updateToolbar();
}

function handleWheel(event) {
  if (state.isImage && event.ctrlKey) {
    event.preventDefault();
    const factor = event.deltaY < 0 ? 1.1 : 1 / 1.1;
    zoomAtCursor(event, factor);
    return;
  }

  event.preventDefault();
  if (state.wheelNavTimer) return;
  state.wheelNavTimer = setTimeout(() => {
    state.wheelNavTimer = null;
  }, 180);
  step(event.deltaY > 0 ? 1 : -1).catch(console.error);
}

async function boot() {
  state.album = await GetActiveAlbum();
  if (!state.album) {
    previewMessage.textContent = "No album loaded";
    return;
  }

  state.previewableIndices = buildPreviewableIndices(state.album);
  const startIndex = await GetPreviewIndex();
  state.currentListPos = listPosForAlbumIndex(startIndex);
  await renderCurrent();
}

window.previewGoTo = (albumIndex) => {
  goToAlbumIndex(Number(albumIndex)).catch((error) => {
    previewMessage.hidden = false;
    previewMessage.textContent = error instanceof Error ? error.message : String(error);
  });
};

previewPrev.addEventListener("click", () => {
  step(-1).catch(console.error);
});

previewNext.addEventListener("click", () => {
  step(1).catch(console.error);
});

previewFlipX.addEventListener("click", () => {
  state.flipX = !state.flipX;
  applyImageTransform();
  updateToolbar();
});

previewFlipY.addEventListener("click", () => {
  state.flipY = !state.flipY;
  applyImageTransform();
  updateToolbar();
});

previewViewport.addEventListener(
  "wheel",
  (event) => {
    handleWheel(event);
  },
  { passive: false },
);

previewStage.addEventListener(
  "wheel",
  (event) => {
    if (!state.isImage || event.target === previewViewport || previewViewport.contains(event.target)) {
      return;
    }
    handleWheel(event);
  },
  { passive: false },
);

previewViewport.addEventListener("pointerdown", (event) => {
  if (!state.isImage || event.button !== 0) return;
  state.dragging = true;
  state.dragStartX = event.clientX;
  state.dragStartY = event.clientY;
  state.panStartX = state.panX;
  state.panStartY = state.panY;
  previewViewport.setPointerCapture(event.pointerId);
  previewViewport.classList.add("dragging");
});

previewViewport.addEventListener("pointermove", (event) => {
  if (!state.dragging) return;
  state.panX = state.panStartX + (event.clientX - state.dragStartX);
  state.panY = state.panStartY + (event.clientY - state.dragStartY);
  applyImageTransform();
});

function endDrag(event) {
  if (!state.dragging) return;
  state.dragging = false;
  previewViewport.classList.remove("dragging");
  if (event?.pointerId != null) {
    try {
      previewViewport.releasePointerCapture(event.pointerId);
    } catch {}
  }
}

previewViewport.addEventListener("pointerup", endDrag);
previewViewport.addEventListener("pointercancel", endDrag);

window.addEventListener("resize", () => {
  const img = previewMediaLayer.querySelector("img");
  if (img && state.isImage) {
    fitImageToViewport(img);
  }
});

document.addEventListener("keydown", (event) => {
  if (event.key === "ArrowLeft") {
    event.preventDefault();
    step(-1).catch(console.error);
  } else if (event.key === "ArrowRight") {
    event.preventDefault();
    step(1).catch(console.error);
  } else if (event.key === "Escape") {
    window.close();
  }
});

boot().catch((error) => {
  previewMessage.hidden = false;
  previewMessage.textContent = error instanceof Error ? error.message : String(error);
});
