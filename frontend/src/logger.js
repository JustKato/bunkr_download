import { Events } from "/wails/runtime.js";
import { AppendConsoleLog } from "../bindings/github.com/justkato/bunkr_download/bunkrservice.js";

let installed = false;
let forwarding = false;

const nativeConsole = {
  log: console.log.bind(console),
  info: (console.info || console.log).bind(console),
  warn: console.warn.bind(console),
  error: console.error.bind(console),
  debug: (console.debug || console.log).bind(console),
};

const recentLocalLogs = new Set();

function formatArg(arg) {
  if (arg instanceof Error) {
    return arg.stack || arg.message;
  }
  if (typeof arg === "object") {
    try {
      return JSON.stringify(arg);
    } catch {
      return String(arg);
    }
  }
  return String(arg);
}

function normalizeLevel(level) {
  switch (String(level || "info").toLowerCase()) {
    case "debug":
      return "debug";
    case "warn":
    case "warning":
      return "warn";
    case "error":
      return "error";
    default:
      return "info";
  }
}

function logKey(level, source, message) {
  return `${normalizeLevel(level)}|${source}|${message}`;
}

function markLocalLog(level, source, message) {
  const key = logKey(level, source, message);
  recentLocalLogs.add(key);
  setTimeout(() => recentLocalLogs.delete(key), 250);
}

function shouldSkipEventMirror(entry) {
  return recentLocalLogs.has(
    logKey(entry.level || "info", entry.source || "?", entry.message || ""),
  );
}

function mirrorToNative(level, source, message) {
  const normalized = normalizeLevel(level);
  const text = String(message ?? "");
  const fn =
    normalized === "error"
      ? nativeConsole.error
      : normalized === "warn"
        ? nativeConsole.warn
        : normalized === "debug"
          ? nativeConsole.debug
          : nativeConsole.log;
  fn(`[${source}]`, text);
}

function sendToBackend(level, source, message) {
  markLocalLog(level, source, message);
  forwarding = true;
  AppendConsoleLog(normalizeLevel(level), source, message)
    .catch(() => {})
    .finally(() => {
      forwarding = false;
    });
}

function write(level, source, message) {
  if (forwarding) {
    return;
  }

  const normalized = normalizeLevel(level);
  const text = String(message ?? "");
  mirrorToNative(normalized, source, text);
  sendToBackend(normalized, source, text);
}

export const log = {
  debug: (source, message) => write("debug", source, message),
  info: (source, message) => write("info", source, message),
  warn: (source, message) => write("warn", source, message),
  error: (source, message) => write("error", source, message),
};

export function installLogger() {
  if (installed) {
    return;
  }
  installed = true;

  for (const level of ["log", "info", "warn", "error", "debug"]) {
    const original = console[level] || nativeConsole.log;
    console[level] = (...args) => {
      original.apply(console, args);
      if (forwarding) {
        return;
      }
      const mapped = level === "log" ? "info" : level;
      const text = args.map(formatArg).join(" ");
      sendToBackend(mapped, "console", text);
    };
  }

  window.addEventListener("error", (event) => {
    write(
      "error",
      "window",
      `${event.message} at ${event.filename}:${event.lineno}:${event.colno}`,
    );
  });

  window.addEventListener("unhandledrejection", (event) => {
    const reason = event.reason;
    write(
      "error",
      "promise",
      reason instanceof Error ? reason.message : String(reason),
    );
  });

  Events.On("console:log", (event) => {
    const entry = event?.data ?? event;
    if (!entry || typeof entry !== "object" || shouldSkipEventMirror(entry)) {
      return;
    }
    mirrorToNative(entry.level || "info", entry.source || "backend", entry.message || "");
  });

  log.info("logger", "frontend logger installed");
}
