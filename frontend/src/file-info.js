import {
  CloseFileInfo,
  GetFileDetails,
  GetFileInfoIndex,
} from "../bindings/github.com/justkato/bunkr_download/bunkrservice.js";
import { renderFileInfoBody } from "./context-menu.js";

const fileInfoBody = document.getElementById("file-info-body");
const fileInfoCloseBtn = document.getElementById("file-info-close-btn");

async function reloadFileInfo() {
  if (!fileInfoBody) {
    return;
  }

  try {
    const index = await GetFileInfoIndex();
    const details = await GetFileDetails(index);
    renderFileInfoBody(fileInfoBody, details);
    if (details?.name) {
      document.title = `File Info - ${details.name}`;
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    fileInfoBody.replaceChildren();
    const errorText = document.createElement("p");
    errorText.className = "dialog-error";
    errorText.textContent = message;
    fileInfoBody.append(errorText);
  }
}

window.reloadFileInfo = reloadFileInfo;

fileInfoCloseBtn?.addEventListener("click", () => {
  CloseFileInfo();
});

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    CloseFileInfo();
  }
});

reloadFileInfo();
