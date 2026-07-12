import { CloseAbout } from "../bindings/github.com/justkato/bunkr_download/bunkrservice.js";

document.getElementById("about-close-btn")?.addEventListener("click", () => {
  CloseAbout();
});

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    CloseAbout();
  }
});
