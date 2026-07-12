import { Events } from "/wails/runtime.js";
import { AppendConsoleLog } from "../bindings/github.com/justkato/bunkr_download/bunkrservice.js";

let installed = false;

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

function write(level, source, message) {
  const normalized = normalizeLevel(level);
  const text = String(message ?? "");
  const fn =
    normalized === "error"
      ? console.error
      : normalized === "warn"
        ? console.warn
        : console.log;
  fn(`[${source}]`, text);
  AppendConsoleLog(normalized, source, text).catch(() => {});
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
    const original = console[level] || console.log;
    console[level] = (...args) => {
      original.apply(console, args);
      const mapped = level === "log" ? "info" : level;
      write(mapped, "console", args.map(formatArg).join(" "));
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

  Events.On("download:progress", (event) => {
    const progress = event?.data ?? event;
    write("debug", "download", JSON.stringify(progress));
  });

  Events.On("console:log", (event) => {
    const entry = event?.data ?? event;
    if (!entry || typeof entry !== "object") {
      return;
    }
    const level = normalizeLevel(entry.level);
    const fn =
      level === "error"
        ? console.error
        : level === "warn"
          ? console.warn
          : console.log;
    fn(`[${entry.source || "backend"}]`, entry.message || "");
  });

  log.info("logger", "frontend logger installed");
}
