export function initHistoryPicker({ root, trigger, menu, onSelect }) {
  let open = false;

  function close() {
    menu.hidden = true;
    trigger.classList.remove("open");
    open = false;
  }

  function openMenu() {
    menu.hidden = false;
    trigger.classList.add("open");
    open = true;
  }

  function toggle(event) {
    event?.stopPropagation();
    if (trigger.disabled) {
      return;
    }
    if (open) {
      close();
    } else {
      openMenu();
    }
  }

  function render(entries, formatLabel) {
    menu.replaceChildren();

    if (!entries.length) {
      const empty = document.createElement("p");
      empty.className = "history-menu-empty";
      empty.textContent = "No albums yet";
      menu.append(empty);
      return;
    }

    for (const entry of entries) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "history-menu-item";
      button.textContent = formatLabel(entry);
      button.title = entry.url || "";
      button.addEventListener("click", (event) => {
        event.stopPropagation();
        close();
        onSelect(entry.url || "");
      });
      menu.append(button);
    }
  }

  trigger.addEventListener("click", toggle);

  document.addEventListener("click", (event) => {
    if (open && !root.contains(event.target)) {
      close();
    }
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      close();
    }
  });

  return {
    render,
    close,
    setDisabled(disabled) {
      trigger.disabled = disabled;
      if (disabled) {
        close();
      }
    },
  };
}
