import { GetSettings, SaveSettings } from "../bindings/github.com/justkato/bunkr_download/bunkrservice.js";
import { AppSettings } from "../bindings/github.com/justkato/bunkr_download/models.js";

const DEFAULTS = {
  pageSize: 25,
  maxAlbumHistory: 1000,
  maxHttpRetries: 5,
  parallelDownloads: 1,
};

let modal = null;
let advancedVisible = false;
let onApplied = null;

function clampNumber(value, min, max, fallback) {
  const parsed = Number.parseInt(String(value), 10);
  if (Number.isNaN(parsed)) {
    return fallback;
  }
  return Math.min(max, Math.max(min, parsed));
}

function field(id) {
  return document.getElementById(id);
}

function readCheckbox(id) {
  return !!field(id)?.checked;
}

function writeCheckbox(id, value) {
  const input = field(id);
  if (input) {
    input.checked = !!value;
  }
}

function readNumber(id, min, max, fallback) {
  return clampNumber(field(id)?.value, min, max, fallback);
}

function writeNumber(id, value) {
  const input = field(id);
  if (input) {
    input.value = String(value ?? "");
  }
}

function buildSettingsFromModal(currentSettings) {
  return new AppSettings({
    outputFolder: currentSettings.outputFolder || "",
    filterTypes: currentSettings.filterTypes || [],
    includePatterns: currentSettings.includePatterns || "",
    paginationMode: currentSettings.paginationMode || "pagination",
    viewMode: currentSettings.viewMode || "list",
    openOutputFolderOnComplete: readCheckbox("opt-open-folder-on-complete"),
    pageSize: readNumber("opt-page-size", 10, 100, DEFAULTS.pageSize),
    maxAlbumHistory: readNumber("opt-max-history", 50, 5000, DEFAULTS.maxAlbumHistory),
    skipExistingFiles: readCheckbox("opt-skip-existing"),
    createAlbumSubfolder: readCheckbox("opt-create-subfolder"),
    continueOnFileFailure: readCheckbox("opt-continue-on-failure"),
    maxHttpRetries: readNumber("opt-max-retries", 0, 10, DEFAULTS.maxHttpRetries),
    parallelDownloads: readNumber("opt-parallel-downloads", 1, 8, DEFAULTS.parallelDownloads),
  });
}

function applySettingsToModal(settings) {
  writeCheckbox("opt-open-folder-on-complete", settings.openOutputFolderOnComplete);
  writeNumber("opt-page-size", settings.pageSize || DEFAULTS.pageSize);
  writeNumber("opt-max-history", settings.maxAlbumHistory || DEFAULTS.maxAlbumHistory);
  writeCheckbox("opt-skip-existing", settings.skipExistingFiles !== false);
  writeCheckbox("opt-create-subfolder", settings.createAlbumSubfolder !== false);
  writeCheckbox("opt-continue-on-failure", settings.continueOnFileFailure);
  writeNumber("opt-max-retries", settings.maxHttpRetries ?? DEFAULTS.maxHttpRetries);
  writeNumber("opt-parallel-downloads", settings.parallelDownloads || DEFAULTS.parallelDownloads);
}

function setAdvancedVisible(visible) {
  advancedVisible = visible;
  const panel = field("options-advanced-panel");
  const toggle = field("options-advanced-toggle");
  if (panel) {
    panel.hidden = !visible;
  }
  if (toggle) {
    toggle.textContent = visible ? "Hide Advanced Options" : "Show Advanced Options";
    toggle.setAttribute("aria-expanded", visible ? "true" : "false");
  }
}

function closeModal() {
  if (!modal) {
    return;
  }
  modal.hidden = true;
}

export async function openOptionsModal({ applySettings }) {
  modal = field("options-modal");
  if (!modal) {
    return;
  }

  onApplied = applySettings;
  setAdvancedVisible(advancedVisible);

  try {
    const settings = await GetSettings();
    applySettingsToModal(settings);
  } catch {
    applySettingsToModal(new AppSettings({}));
  }

  modal.hidden = false;
  field("opt-open-folder-on-complete")?.focus();
}

export function initOptionsModal() {
  modal = field("options-modal");
  if (!modal) {
    return;
  }

  field("options-close-btn")?.addEventListener("click", closeModal);
  field("options-cancel-btn")?.addEventListener("click", closeModal);
  field("options-advanced-toggle")?.addEventListener("click", () => {
    setAdvancedVisible(!advancedVisible);
  });

  modal.addEventListener("click", (event) => {
    if (event.target === modal) {
      closeModal();
    }
  });

  field("options-save-btn")?.addEventListener("click", async () => {
    try {
      const current = await GetSettings();
      const next = buildSettingsFromModal(current);
      await SaveSettings(next);
      onApplied?.(next);
      closeModal();
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      field("options-status")?.replaceChildren(document.createTextNode(message));
    }
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && modal && !modal.hidden) {
      closeModal();
    }
  });
}
