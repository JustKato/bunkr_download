import { Events } from "/wails/runtime.js";
import {
  ClearConsoleLogs,
  CloseConsole,
  GetConsoleLogs,
} from "../bindings/github.com/justkato/bunkr_download/bunkrservice.js";

const consoleLog = document.getElementById("console-log");
const copyBtn = document.getElementById("console-copy-btn");
const clearBtn = document.getElementById("console-clear-btn");
const closeBtn = document.getElementById("console-close-btn");

const entries = [];

function formatTime(ms) {
  try {
    return new Date(ms).toLocaleTimeString();
  } catch {
    return "--:--:--";
  }
}

function renderEntry(entry) {
  const line = document.createElement("div");
  line.className = `console-line level-${entry.level || "info"}`;
  line.textContent = `[${formatTime(entry.time)}] [${entry.level || "info"}] [${entry.source || "?"}] ${entry.message || ""}`;
  return line;
}

function renderAll() {
  consoleLog.replaceChildren();
  for (const entry of entries) {
    consoleLog.append(renderEntry(entry));
  }
  consoleLog.scrollTop = consoleLog.scrollHeight;
}

function appendEntry(entry) {
  if (!entry || typeof entry !== "object") {
    return;
  }
  entries.push(entry);
  if (entries.length > 2000) {
    entries.splice(0, entries.length - 2000);
  }
  consoleLog.append(renderEntry(entry));
  consoleLog.scrollTop = consoleLog.scrollHeight;
}

function entryText(entry) {
  return `[${formatTime(entry.time)}] [${entry.level || "info"}] [${entry.source || "?"}] ${entry.message || ""}`;
}

async function reloadConsole() {
  try {
    const logs = await GetConsoleLogs();
    entries.length = 0;
    if (Array.isArray(logs)) {
      entries.push(...logs);
    }
    renderAll();
  } catch (error) {
    consoleLog.textContent =
      error instanceof Error ? error.message : String(error);
  }
}

window.reloadConsole = () => {
  reloadConsole().catch(console.error);
};

copyBtn.addEventListener("click", async () => {
  const text = entries.map(entryText).join("\n");
  try {
    await navigator.clipboard.writeText(text);
  } catch (error) {
    console.error(error);
  }
});

clearBtn.addEventListener("click", async () => {
  try {
    await ClearConsoleLogs();
    entries.length = 0;
    renderAll();
  } catch (error) {
    console.error(error);
  }
});

closeBtn.addEventListener("click", () => {
  CloseConsole().catch(console.error);
});

Events.On("console:log", (event) => {
  appendEntry(event?.data ?? event);
});

reloadConsole().catch(console.error);
