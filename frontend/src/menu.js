export function initMenu(handlers) {
  const menubar = document.getElementById("menubar");
  let openMenu = null;

  function closeSubmenus(container) {
    const scope = container || document;
    scope.querySelectorAll(".menu-submenu").forEach((menu) => {
      menu.hidden = true;
    });
    scope.querySelectorAll(".menu-submenu-trigger").forEach((trigger) => {
      trigger.classList.remove("open");
    });
  }

  function closeMenus() {
    document.querySelectorAll(".menu-dropdown").forEach((menu) => {
      menu.hidden = true;
    });
    document.querySelectorAll(".menu-trigger").forEach((trigger) => {
      trigger.classList.remove("open");
    });
    closeSubmenus();
    openMenu = null;
  }

  document.querySelectorAll(".menu-trigger").forEach((trigger) => {
    trigger.addEventListener("click", (event) => {
      event.stopPropagation();
      const menuID = trigger.dataset.menu;
      const menu = document.getElementById(menuID);
      if (!menu) return;

      if (openMenu === menu) {
        closeMenus();
        return;
      }

      closeMenus();
      menu.hidden = false;
      trigger.classList.add("open");
      openMenu = menu;
    });
  });

  document.querySelectorAll(".menu-submenu-trigger").forEach((trigger) => {
    trigger.addEventListener("click", (event) => {
      event.stopPropagation();
      const submenuID = trigger.dataset.submenu;
      const submenu = document.getElementById(submenuID);
      if (!submenu) return;

      const parentMenu = trigger.closest(".menu-dropdown");
      closeSubmenus(parentMenu);

      const willOpen = submenu.hidden;
      submenu.hidden = !willOpen;
      trigger.classList.toggle("open", willOpen);
    });
  });

  document.querySelectorAll(".menu-dropdown button[data-action]").forEach((item) => {
    item.addEventListener("click", () => {
      const action = item.dataset.action;
      closeMenus();
      handlers[action]?.();
    });
  });

  document.addEventListener("click", (event) => {
    if (!menubar.contains(event.target)) {
      closeMenus();
    }
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      closeMenus();
    }
  });

  return { closeMenus };
}
