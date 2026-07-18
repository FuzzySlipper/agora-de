import type { SettingsPageDefinition } from '@agora-de/domain';

export function generateSettingsHostHTML(pages: readonly SettingsPageDefinition[]): string {
  const templates = pages
    .map(
      (page) =>
        `<template data-settings-entry="${escapeAttribute(page.uiEntryPoint)}">${page.renderPanel()}</template>`,
    )
    .join('\n');
  const controllers = pages
    .filter((page) => page.renderClientController)
    .map((page) => `${JSON.stringify(page.uiEntryPoint)}: ${safeControllerSource(page.renderClientController!())}`)
    .join(',\n');

  return `<!doctype html>
<html>
<head>
  <title>Agora Settings</title>
  <meta name="color-scheme" content="dark">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>
__AGORA_THEME_CSS__
    * { box-sizing: border-box; }
    html, body { background: var(--agora-bg); color: var(--agora-fg); font: var(--agora-font-body); height: 100%; margin: 0; }
    body { display: grid; grid-template-rows: auto minmax(0, 1fr) auto; min-height: 100vh; overflow: hidden; }
    button, input { font: inherit; }
    button:focus-visible, input:focus-visible { outline: 3px solid var(--agora-accent); outline-offset: 2px; }
    .visually-hidden { height: 1px; margin: -1px; overflow: hidden; padding: 0; position: absolute; width: 1px; clip: rect(0 0 0 0); white-space: nowrap; }
    .app-header { align-items: center; border-bottom: 1px solid var(--agora-border-subtle); display: flex; gap: 14px; min-height: 72px; padding: 14px 20px; }
    .mark { background: var(--agora-evidence-accent); border-radius: var(--agora-radius-control); height: 34px; width: 34px; }
    .title-block { min-width: 0; }
    h1, h2, h3, p { margin: 0; }
    h1 { font-size: 22px; }
    .subtitle { color: var(--agora-text-muted); font-size: 13px; margin-top: 3px; }
    .settings-search { background: var(--agora-surface); border: 2px solid var(--agora-border); border-radius: var(--agora-radius-control); color: var(--agora-fg); margin-left: auto; min-height: 42px; min-width: min(340px, 42vw); padding: 0 13px; }
    .settings-layout { display: grid; grid-template-columns: minmax(220px, 29%) minmax(0, 1fr); min-height: 0; }
    .sidebar { background: var(--agora-surface); border-right: 1px solid var(--agora-border-subtle); min-height: 0; overflow-y: auto; padding: 14px; }
    .category { margin-bottom: 18px; }
    .category-title { color: var(--agora-text-muted); font-size: 12px; font-weight: 800; letter-spacing: .08em; margin: 4px 8px 8px; text-transform: uppercase; }
    .module-button { background: transparent; border: 1px solid transparent; border-radius: var(--agora-radius-control); color: var(--agora-fg); cursor: pointer; display: grid; gap: 3px; min-height: 58px; padding: 9px 10px; text-align: left; width: 100%; }
    .module-button:hover { background: var(--agora-surface-raised); }
    .module-button[aria-current="page"] { background: var(--agora-surface-raised); border-color: var(--agora-accent); }
    .module-button-title { font-weight: 800; }
    .module-button-summary { color: var(--agora-text-muted); font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .module-button-state { color: var(--agora-warning); font-size: 11px; }
    .content { min-height: 0; overflow-y: auto; padding: clamp(18px, 4vw, 38px); }
    .content-header { border-bottom: 1px solid var(--agora-border-subtle); margin-bottom: 22px; padding-bottom: 16px; }
    .content-title { font-size: 24px; }
    .content-summary { color: var(--agora-text-muted); margin-top: 7px; max-width: 70ch; }
    .module-content { display: grid; gap: 14px; }
    .settings-module-panel { display: grid; gap: 14px; }
    .setting-row { align-items: center; background: var(--agora-surface); border: 1px solid var(--agora-border); border-radius: var(--agora-radius-control); display: grid; gap: 14px; grid-template-columns: minmax(0, 1fr) auto; min-height: 92px; padding: 16px 18px; }
    .setting-copy { display: grid; gap: 6px; min-width: 0; }
    .setting-title { font-size: 16px; }
    .setting-detail { color: var(--agora-text-muted); font-size: 13px; }
    .toggle { align-items: center; background: var(--agora-surface-raised); border: 2px solid var(--agora-border); border-radius: 999px; color: var(--agora-fg); cursor: pointer; display: inline-flex; height: 38px; justify-content: flex-start; min-width: 76px; padding: 0 5px; }
    .toggle::before { background: var(--agora-text-muted); border-radius: 50%; content: ""; height: 24px; transition: transform var(--agora-duration) var(--agora-ease), background var(--agora-duration) var(--agora-ease); width: 24px; }
    .toggle[aria-pressed="true"] { border-color: var(--agora-accent); }
    .toggle[aria-pressed="true"]::before { background: var(--agora-accent); transform: translateX(36px); }
    .empty, .module-error { background: var(--agora-surface); border: 1px solid var(--agora-border); border-radius: var(--agora-radius-control); color: var(--agora-text-muted); padding: 20px; }
    .module-error { border-color: var(--agora-warning); color: var(--agora-fg); }
    .error-title { color: var(--agora-warning); font-weight: 800; }
    .error-detail { margin-top: 6px; }
    .validation-list { color: var(--agora-warning); margin: 10px 0 0; padding-left: 22px; }
    .app-footer { align-items: center; border-top: 1px solid var(--agora-border-subtle); display: flex; gap: 10px; min-height: 66px; padding: 12px 20px; }
    .status { color: var(--agora-text-muted); margin-right: auto; }
    .dirty-indicator { color: var(--agora-warning); font-weight: 800; }
    .action { background: var(--agora-surface-raised); border: 2px solid var(--agora-border); border-radius: var(--agora-radius-control); color: var(--agora-fg); cursor: pointer; min-height: 40px; padding: 0 15px; }
    .action.primary { background: var(--agora-accent); border-color: var(--agora-accent); color: var(--agora-surface-strong); font-weight: 800; }
    .action:disabled, .toggle:disabled { cursor: default; opacity: .5; }
    @media (max-width: 720px) {
      .app-header { align-items: stretch; flex-wrap: wrap; }
      .settings-search { flex-basis: 100%; margin-left: 0; min-width: 0; width: 100%; }
      .settings-layout { grid-template-columns: 1fr; grid-template-rows: auto minmax(0, 1fr); }
      .sidebar { border-bottom: 1px solid var(--agora-border-subtle); border-right: 0; display: flex; gap: 8px; max-height: 150px; overflow: auto; }
      .category { display: flex; flex: 0 0 auto; gap: 6px; margin: 0; }
      .category-title { align-self: center; margin: 0 4px; }
      .module-button { min-width: 190px; }
      .content { padding: 18px; }
      .app-footer { flex-wrap: wrap; }
      .status { flex-basis: 100%; }
    }
    @media (prefers-reduced-motion: reduce) {
      *, *::before, *::after { scroll-behavior: auto !important; transition-duration: 0.001ms !important; }
    }
  </style>
</head>
<body data-surface="settings">
  <header class="app-header">
    <span class="mark" aria-hidden="true"></span>
    <div class="title-block"><h1>Settings</h1><p class="subtitle">Configure your Agora desktop</p></div>
    <label for="settings-search" class="visually-hidden">Search settings</label>
    <input class="settings-search" id="settings-search" type="search" placeholder="Search settings" autocomplete="off">
  </header>
  <div class="settings-layout">
    <nav class="sidebar" id="settings-navigation" aria-label="Settings categories"></nav>
    <main class="content" id="settings-content" tabindex="-1">
      <header class="content-header"><h2 class="content-title" id="module-title">Settings</h2><p class="content-summary" id="module-summary">Loading modules…</p></header>
      <section class="module-content" id="module-content" aria-live="polite"></section>
    </main>
  </div>
  <footer class="app-footer">
    <span class="status" id="settings-status" role="status">Loading settings…</span>
    <span class="dirty-indicator" id="dirty-indicator" hidden>Unsaved changes</span>
    <button class="action" id="defaults-button" type="button" disabled>Restore Defaults</button>
    <button class="action" id="reset-button" type="button" disabled>Reset</button>
    <button class="action primary" id="apply-button" type="button" disabled>Apply</button>
  </footer>
  ${templates}
  <script>
    const controllerFactories = Object.freeze({${controllers}});
    const state = { catalog: {schemaVersion: 1, modules: []}, selectedId: "", moduleState: null, draft: null, dirty: false, busy: false, error: null, notice: "", controller: null };
    const navigation = document.getElementById("settings-navigation");
    const content = document.getElementById("module-content");
    const title = document.getElementById("module-title");
    const summary = document.getElementById("module-summary");
    const status = document.getElementById("settings-status");
    const search = document.getElementById("settings-search");

    function clone(value) { return value === undefined ? undefined : JSON.parse(JSON.stringify(value)); }
    function selectedEntry() { return state.catalog.modules.find((entry) => entry.manifest.id === state.selectedId); }
    function operationPath(moduleId, operation) { return "/api/settings/modules/" + moduleId + "/" + operation; }
    function text(value, fallback) { const normalized = String(value ?? "").trim(); return normalized || fallback; }
    function valueAt(source, path) { return String(path || "").split(".").reduce((value, key) => value == null ? undefined : value[key], source); }
    function setValueAt(target, path, value) { const keys = String(path).split("."); let cursor = target; keys.slice(0, -1).forEach((key) => { cursor[key] = cursor[key] || {}; cursor = cursor[key]; }); cursor[keys[keys.length - 1]] = value; }

    async function requestJSON(path, options) {
      const response = await fetch(path, Object.assign({cache: "no-store"}, options || {}));
      let payload = {};
      try { payload = await response.json(); } catch (_error) { payload = {}; }
      if (!response.ok) { const error = new Error(text(payload.message, path + " returned " + response.status)); error.settings = payload; throw error; }
      return payload;
    }
    function postJSON(path, body) { return requestJSON(path, {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(body)}); }

    function filteredModules() {
      const query = search.value.trim().toLocaleLowerCase();
      if (!query) return state.catalog.modules;
      return state.catalog.modules.filter((entry) => {
        const manifest = entry.manifest;
        return [manifest.title, manifest.summary, manifest.category].concat(manifest.searchTerms || []).join(" ").toLocaleLowerCase().includes(query);
      });
    }

    function renderNavigation() {
      const groups = new Map();
      filteredModules().forEach((entry) => {
        const category = text(entry.manifest.category, "system");
        if (!groups.has(category)) groups.set(category, []);
        groups.get(category).push(entry);
      });
      navigation.replaceChildren();
      groups.forEach((entries, category) => {
        const section = document.createElement("section"); section.className = "category";
        const heading = document.createElement("h2"); heading.className = "category-title"; heading.textContent = category; section.appendChild(heading);
        entries.forEach((entry) => {
          const button = document.createElement("button"); button.type = "button"; button.className = "module-button"; button.dataset.moduleId = entry.manifest.id;
          if (entry.manifest.id === state.selectedId) button.setAttribute("aria-current", "page");
          const label = document.createElement("span"); label.className = "module-button-title"; label.textContent = entry.manifest.title; button.appendChild(label);
          const detail = document.createElement("span"); detail.className = "module-button-summary"; detail.textContent = entry.manifest.summary; button.appendChild(detail);
          if (entry.availability.state !== "available") { const unavailable = document.createElement("span"); unavailable.className = "module-button-state"; unavailable.textContent = entry.availability.state.replaceAll("_", " "); button.appendChild(unavailable); }
          button.addEventListener("click", () => selectModule(entry.manifest.id, true)); section.appendChild(button);
        });
        navigation.appendChild(section);
      });
      if (!navigation.children.length) { const empty = document.createElement("p"); empty.className = "empty"; empty.textContent = "No settings match your search."; navigation.appendChild(empty); }
    }

    function findTemplate(uiEntryPoint) { return Array.from(document.querySelectorAll("template[data-settings-entry]")).find((template) => template.dataset.settingsEntry === uiEntryPoint); }
    function bindModuleControls() {
      content.querySelectorAll("[data-settings-field]").forEach((control) => {
        control.addEventListener("click", () => {
          if (state.busy || !state.draft) return;
          const field = control.dataset.settingsField;
          setValueAt(state.draft, field, !Boolean(valueAt(state.draft, field)));
          state.dirty = true; state.error = null; state.notice = ""; content.querySelector(".validation-list")?.remove(); renderModuleState();
        });
      });
    }

    function activatePageController(entry) {
      const factory = controllerFactories[entry.manifest.uiEntryPoint];
      if (!factory) { state.controller = null; return; }
      state.controller = factory({
        content,
        getModuleState: () => state.moduleState,
        getDraft: () => state.draft,
        isDirty: () => state.dirty,
        load: () => requestJSON(operationPath(entry.manifest.id, "load")),
        markDirty: () => { state.dirty = true; state.error = null; state.notice = ""; renderModuleState(); },
        post: (operation, body) => postJSON(operationPath(entry.manifest.id, operation), body),
        acceptResult: (result, notice) => { state.moduleState = result.state; state.draft = clone(result.state.active); state.dirty = false; state.error = null; state.notice = notice || "Changes applied"; renderModuleState(); },
        acceptExternalState: (moduleState, draft, dirty, notice) => { state.moduleState = moduleState; state.draft = draft; state.dirty = dirty; state.error = null; state.notice = notice || "Display topology changed"; renderModuleState(); },
        setError: (error) => { state.error = error; renderModuleState(); },
        reload: loadSelectedModule,
      }) || null;
    }

    function renderModuleState() {
      content.querySelectorAll("[data-settings-field]").forEach((control) => {
        const enabled = Boolean(valueAt(state.draft, control.dataset.settingsField));
        control.setAttribute("aria-pressed", String(enabled)); control.disabled = state.busy || !state.moduleState;
      });
      content.querySelectorAll("[data-settings-text]").forEach((element) => { element.textContent = text(valueAt(state.moduleState, element.dataset.settingsText), "unknown"); });
      document.getElementById("dirty-indicator").hidden = !state.dirty;
      document.getElementById("apply-button").disabled = state.busy || !state.dirty || !state.moduleState;
      document.getElementById("reset-button").disabled = state.busy || !state.dirty || !state.moduleState;
      document.getElementById("defaults-button").disabled = state.busy || !state.moduleState;
      status.textContent = state.busy ? "Working…" : state.error ? text(state.error.message, "Settings operation failed") : state.notice ? state.notice : state.dirty ? "Review and apply your changes" : state.moduleState ? "Up to date" : "Loading…";
      if (state.controller && state.controller.render) state.controller.render();
    }

    function renderError(error, retryable) {
      const box = document.createElement("section"); box.className = "module-error"; box.setAttribute("role", "alert");
      const heading = document.createElement("h3"); heading.className = "error-title"; heading.textContent = text(error && error.code, "Settings unavailable").replaceAll("_", " "); box.appendChild(heading);
      const detail = document.createElement("p"); detail.className = "error-detail"; detail.textContent = text(error && error.message, "This module could not be loaded."); box.appendChild(detail);
      if (retryable) { const retry = document.createElement("button"); retry.className = "action"; retry.type = "button"; retry.textContent = "Retry"; retry.addEventListener("click", loadSelectedModule); box.appendChild(retry); }
      content.replaceChildren(box); state.error = error; state.moduleState = null; state.draft = null; state.dirty = false; renderModuleState();
    }

    async function selectModule(moduleId, pushHistory) {
      if (moduleId === state.selectedId && state.moduleState) return;
      if (state.dirty && !window.confirm("Discard unsaved settings changes?")) { renderNavigation(); return; }
      const entry = state.catalog.modules.find((candidate) => candidate.manifest.id === moduleId);
      if (!entry) return;
      if (state.controller && state.controller.destroy) state.controller.destroy();
      state.selectedId = moduleId; state.moduleState = null; state.draft = null; state.dirty = false; state.error = null; state.notice = "";
      if (pushHistory) { const url = new URL(window.location.href); url.searchParams.set("module", moduleId); history.pushState({moduleId}, "", url); }
      title.textContent = entry.manifest.title; summary.textContent = entry.manifest.summary; renderNavigation();
      const template = findTemplate(entry.manifest.uiEntryPoint);
      if (!template) { renderError({code: "unsupported", message: "This build does not include the module page."}, false); return; }
      content.replaceChildren(template.content.cloneNode(true)); bindModuleControls(); activatePageController(entry);
      if (entry.availability.state === "unsupported" || entry.availability.state === "unavailable") { renderError({code: entry.availability.state, message: text(entry.availability.reason, "The module backend is unavailable.")}, true); return; }
      await loadSelectedModule();
    }

    async function loadSelectedModule() {
      const entry = selectedEntry(); if (!entry) return;
      state.busy = true; state.error = null; state.notice = ""; renderModuleState();
      try {
        state.moduleState = await requestJSON(operationPath(entry.manifest.id, "load"));
        state.draft = clone(state.moduleState.active); state.dirty = false;
      } catch (error) { renderError(error.settings || {code: "unavailable", message: error.message}, true); return; }
      finally { state.busy = false; }
      renderModuleState();
    }

    async function applyDraft() {
      const entry = selectedEntry(); if (!entry || !state.moduleState || !state.draft || state.busy) return;
      state.busy = true; state.error = null; state.notice = ""; content.querySelector(".validation-list")?.remove(); renderModuleState();
      const request = {contractVersion: entry.manifest.contractVersion, baseRevision: state.moduleState.revision, draft: state.draft};
      try {
        if (state.controller && state.controller.prepareApply) state.controller.prepareApply(request);
        const validation = await postJSON(operationPath(entry.manifest.id, "validate"), request);
        if (!validation.valid) {
          const list = document.createElement("ul"); list.className = "validation-list"; list.setAttribute("role", "alert");
          (validation.issues || []).forEach((issue) => { const item = document.createElement("li"); item.textContent = text(issue.message, "Invalid value"); list.appendChild(item); });
          content.prepend(list); state.error = {code: "validation_failed", message: "Some values need attention."}; return;
        }
        const result = await postJSON(operationPath(entry.manifest.id, "apply"), request);
        state.moduleState = result.state; state.draft = clone(result.state.active); state.dirty = false;
        state.notice = result.outcome && result.outcome.kind === "restart_required" ? "Applied; restart required" : result.outcome && result.outcome.kind === "pending_confirmation" ? "Confirm the new display configuration" : "Changes applied";
        if (state.controller && state.controller.applied) state.controller.applied(result);
      } catch (error) { state.error = error.settings || {code: "apply_failed", message: error.message}; }
      finally { state.busy = false; renderModuleState(); }
    }

    function resetDraft() { if (!state.moduleState || state.busy) return; state.draft = clone(state.moduleState.active); state.dirty = false; state.error = null; state.notice = "Draft reset"; content.querySelector(".validation-list")?.remove(); renderModuleState(); }
    async function restoreDefaults() {
      const entry = selectedEntry(); if (!entry || !state.moduleState || state.busy) return;
      state.busy = true; state.notice = ""; renderModuleState();
      try { state.draft = await postJSON(operationPath(entry.manifest.id, "restore_defaults"), {contractVersion: entry.manifest.contractVersion, baseRevision: state.moduleState.revision}); state.dirty = true; state.error = null; }
      catch (error) { state.error = error.settings || {code: "apply_failed", message: error.message}; }
      finally { state.busy = false; renderModuleState(); }
    }

    async function start() {
      try {
        state.catalog = await requestJSON("/api/settings/catalog"); renderNavigation();
        const requested = new URLSearchParams(location.search).get("module");
        const initial = state.catalog.modules.some((entry) => entry.manifest.id === requested) ? requested : state.catalog.modules[0] && state.catalog.modules[0].manifest.id;
        if (initial) await selectModule(initial, false); else { summary.textContent = "No settings modules are registered."; content.innerHTML = '<p class="empty">No settings modules are available.</p>'; }
      } catch (error) { renderError(error.settings || {code: "unavailable", message: error.message}, true); }
    }

    search.addEventListener("input", renderNavigation);
    navigation.addEventListener("keydown", (event) => { if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return; const buttons = Array.from(navigation.querySelectorAll(".module-button")); const index = buttons.indexOf(document.activeElement); if (index < 0) return; event.preventDefault(); buttons[(index + (event.key === "ArrowDown" ? 1 : -1) + buttons.length) % buttons.length].focus(); });
    document.getElementById("apply-button").addEventListener("click", applyDraft);
    document.getElementById("reset-button").addEventListener("click", resetDraft);
    document.getElementById("defaults-button").addEventListener("click", restoreDefaults);
    window.addEventListener("popstate", () => { const requested = new URLSearchParams(location.search).get("module"); if (requested) selectModule(requested, false); });
    window.addEventListener("beforeunload", (event) => { if (!state.dirty) return; event.preventDefault(); event.returnValue = ""; });
    start();
  </script>
</body>
</html>`;
}

function escapeAttribute(value: string): string {
  return value.replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;');
}

function safeControllerSource(source: string): string {
  const trimmed = source.trim();
  if (!trimmed.startsWith('(') || !trimmed.endsWith('}')) {
    throw new Error('settings page controller must be a parenthesized function expression');
  }
  return trimmed.replaceAll('</script', '<\\/script');
}
