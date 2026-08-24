/* eslint-disable */
// Auto-generated one-time port from go/internal/shellui/server/server.go inline writers.
// This file is the rendering authority for shell surface HTML. The Go shellAssetHandler
// serves these templates (or a StaticRoot override) and substitutes __AGORA_THEME_CSS__
// and __AGORA_SURFACE__ placeholders at runtime, and inlines the shared component CSS.

import { appFallbackIconSVG, componentCSS, shellIconSVG } from '@agora-de/components';
import { liveThemeClientScript } from '@agora-de/theme';

const liveThemeScript = liveThemeClientScript();

export const overlayHTML: string = `<!doctype html>
<html>
<head>
  <title>agora-de agent overlay</title>
  <meta name="color-scheme" content="dark">
  <style>
__AGORA_THEME_CSS__
${componentCSS}
    html,
    body {
      background: transparent !important;
      color: var(--agora-fg);
      height: 100%;
      margin: 0;
      overflow: hidden;
      width: 100%;
    }
    body {
      font: var(--agora-font-status);
      pointer-events: none;
    }
    .agent-overlay {
      height: 100vh;
      inset: 0;
      overflow: hidden;
      pointer-events: none;
      position: fixed;
      width: 100vw;
    }
    .zone-hints {
      display: grid;
      grid-template-columns: 1fr 1fr;
      height: calc(100vh - var(--agora-panel-height));
      inset: 0 0 var(--agora-panel-height) 0;
      opacity: 0.14;
      position: absolute;
    }
    .zone-hint {
      border: 2px dashed var(--agora-border);
      box-sizing: border-box;
      color: var(--agora-text-muted);
      padding: 12px;
      text-transform: uppercase;
    }
    .window-box {
      border: 3px solid var(--agora-evidence-accent);
      box-shadow:
        0 0 0 1px var(--agora-surface-strong),
        inset 0 0 0 1px var(--agora-surface-strong);
      box-sizing: border-box;
      min-height: 72px;
      min-width: 120px;
      position: absolute;
    }
    .window-box.focused {
      border-color: var(--agora-warning);
      box-shadow:
        0 0 0 2px var(--agora-surface-strong),
        var(--agora-focus-glow),
        inset 0 0 0 2px var(--agora-warning);
    }
    .label {
      align-items: center;
      background: var(--agora-overlay-label-bg);
      border: 2px solid var(--agora-evidence-accent);
      color: var(--agora-fg);
      display: inline-flex;
      gap: 8px;
      left: auto;
      max-width: calc(100% - 16px);
      min-height: 36px;
      padding: 0 10px;
      position: absolute;
      right: 8px;
      top: 8px;
      white-space: nowrap;
    }
    .focused .label {
      border-color: var(--agora-warning);
    }
    .number {
      align-items: center;
      background: var(--agora-evidence-accent);
      color: var(--agora-surface-strong);
      display: inline-flex;
      font: var(--agora-font-code);
      height: 24px;
      justify-content: center;
      min-width: 24px;
      padding: 0 4px;
    }
    .copy {
      display: block;
      max-width: 320px;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .bounds {
      background: var(--agora-overlay-chip-bg);
      border: 2px solid var(--agora-border);
      bottom: 8px;
      color: var(--agora-text-muted);
      font: var(--agora-font-code);
      left: 8px;
      padding: 5px 8px;
      position: absolute;
    }
    .meta {
      align-items: center;
      bottom: 8px;
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      max-width: calc(100% - 16px);
      opacity: 0.62;
      position: absolute;
      right: 8px;
    }
    .chip {
      background: var(--agora-overlay-chip-bg);
      border: 2px solid var(--agora-border);
      color: var(--agora-fg);
      font: var(--agora-font-code);
      padding: 5px 7px;
    }
    .action-hints {
      border-color: var(--agora-evidence-accent);
      color: var(--agora-text-muted);
    }
    .fallback-stack {
      display: grid;
      gap: 10px;
      left: 24px;
      max-width: 520px;
      position: absolute;
      top: 24px;
    }
    .fallback-item {
      background: var(--agora-surface-strong);
      border: 2px solid var(--agora-evidence-accent);
      color: var(--agora-fg);
      padding: 10px 12px;
    }
    .empty {
      background: var(--agora-surface-strong);
      border: 2px solid var(--agora-border);
      color: var(--agora-text-muted);
      left: 24px;
      padding: 10px 12px;
      position: absolute;
      top: 24px;
    }
  </style>
</head>
<body data-surface="overlay">
  <main class="agent-overlay" id="agent-overlay-surface" aria-label="Agent-visible window labels and bounds overlay">
    <section class="zone-hints" id="zone-hints" aria-label="Workspace zone hints">
      <span class="zone-hint">primary</span>
      <span class="zone-hint">secondary</span>
    </section>
    <section id="overlay-labels" aria-label="Surface labels"></section>
  </main>
  <script>
    ${liveThemeScript}
    const state = {
      layout: {mode: "freeform", revision: 0, surfaces: [], workspaces: []},
      surfaces: []
    };

    function text(value, fallback) {
      const trimmed = String(value || "").trim();
      return trimmed || fallback;
    }

    function number(value, fallback) {
      const parsed = Number(value);
      return Number.isFinite(parsed) ? parsed : fallback;
    }

    function geometryFor(surface) {
      const geometry = surface && surface.geometry;
      if (!geometry || typeof geometry !== "object") {
        return null;
      }
      const x = number(geometry.x, 0);
      const y = number(geometry.y, 0);
      const width = number(geometry.width, 0);
      const height = number(geometry.height, 0);
      if (width <= 0 || height <= 0) {
        return null;
      }
      return {x, y, width, height};
    }

    function surfaceName(surface) {
      return text(surface.title, text(surface.appId, surface.surfaceId));
    }

    function activeWorkspace() {
      const workspaces = Array.isArray(state.layout.workspaces) ? state.layout.workspaces : [];
      return workspaces.find((workspace) => workspace.active) || workspaces[0] || {zones: []};
    }

    function layoutRuleLabel() {
      const settings = state.layout.settings || {};
      return text(settings.rule, text(state.layout.mode, "freeform"));
    }

    function renderZoneHints() {
      const target = document.getElementById("zone-hints");
      const zones = Array.isArray(activeWorkspace().zones) ? activeWorkspace().zones : [];
      const zoneLabels = zones.length ? zones.map((zone) => text(zone.id, "zone")) : ["primary", "secondary"];
      target.replaceChildren();
      target.style.gridTemplateColumns = "repeat(" + Math.max(1, zoneLabels.length) + ", 1fr)";
      zoneLabels.forEach((zoneId) => {
        const hint = document.createElement("span");
        hint.className = "zone-hint";
        hint.textContent = zoneId;
        target.appendChild(hint);
      });
    }

    function actionHints(surface) {
      const participation = text(surface.participation, surface.floating ? "floating" : "tiled");
      return participation === "floating" || participation === "transient"
        ? "focus close tile"
        : "focus close float zone reset";
    }

    function chip(value, className) {
      const element = document.createElement("span");
      element.className = "chip" + (className ? " " + className : "");
      element.textContent = value;
      return element;
    }

    function renderBox(surface) {
      const geometry = geometryFor(surface);
      if (!geometry) {
        return null;
      }
      const element = document.createElement("article");
      element.className = "window-box" + (surface.focused ? " focused" : "");
      element.dataset.surfaceId = surface.surfaceId;
      element.dataset.zoneId = text(surface.zoneId, "primary");
      element.dataset.order = String(number(surface.order, 0));
      element.dataset.participation = text(surface.participation, surface.floating ? "floating" : "tiled");
      element.dataset.layoutRule = layoutRuleLabel();
      element.style.left = Math.max(0, geometry.x) + "px";
      element.style.top = Math.max(0, geometry.y) + "px";
      element.style.width = geometry.width + "px";
      element.style.height = geometry.height + "px";

      const label = document.createElement("span");
      label.className = "label";
      const numberBadge = document.createElement("span");
      numberBadge.className = "number";
      numberBadge.textContent = text(surface.label, String(number(surface.order, 0) + 1));
      const copy = document.createElement("span");
      copy.className = "copy";
      copy.textContent = "#" + text(surface.surfaceId, "?") + " / " + surfaceName(surface);
      label.append(numberBadge, copy);

      const bounds = document.createElement("span");
      bounds.className = "bounds";
      bounds.textContent = geometry.x + "," + geometry.y + " " + geometry.width + "x" + geometry.height;
      const meta = document.createElement("span");
      meta.className = "meta";
      meta.append(
        chip("order " + String(number(surface.order, 0))),
        chip(text(surface.zoneId, "primary")),
        chip(text(surface.participation, surface.floating ? "floating" : "tiled")),
        chip(layoutRuleLabel()),
        chip(surface.focused ? "focused" : "unfocused"),
        chip(actionHints(surface), "action-hints")
      );
      element.append(label, bounds, meta);
      return element;
    }

    function renderFallback(surfaces) {
      const stack = document.createElement("section");
      stack.className = "fallback-stack";
      surfaces.forEach((surface, index) => {
        const item = document.createElement("span");
        item.className = "fallback-item";
        item.textContent = text(surface.label, String(index + 1)) + " / " + surfaceName(surface) + " / " + text(surface.zoneId, "primary");
        stack.appendChild(item);
      });
      return stack;
    }

    function render() {
      const target = document.getElementById("overlay-labels");
      target.replaceChildren();
      renderZoneHints();
      const surfaces = (Array.isArray(state.layout.surfaces) ? state.layout.surfaces : [])
        .filter((surface) => surface && surface.visible !== false);
      if (!surfaces.length) {
        const empty = document.createElement("span");
        empty.className = "empty";
        empty.textContent = "no work surfaces";
        target.appendChild(empty);
        return;
      }
      const boxes = surfaces.map(renderBox).filter(Boolean);
      if (boxes.length) {
        boxes.forEach((box) => target.appendChild(box));
        return;
      }
      target.appendChild(renderFallback(surfaces));
    }

    async function loadJSON(path) {
      const response = await fetch(path, {cache: "no-store"});
      if (!response.ok) {
        throw new Error(path + " returned " + response.status);
      }
      return response.json();
    }

    async function refresh() {
      try {
        const [layout, surfaces] = await Promise.all([
          loadJSON("/api/layout"),
          loadJSON("/api/surfaces")
        ]);
        state.layout = layout.layout || state.layout;
        state.surfaces = Array.isArray(surfaces.surfaces) ? surfaces.surfaces : [];
        render();
      } catch (error) {
        render();
      }
    }

    refresh();
    setInterval(refresh, 1000);
  </script>
</body>
</html>`;

export const operatorHTML: string = `<!doctype html>
<html>
<head>
  <title>agora-de shell status</title>
  <meta name="color-scheme" content="light">
  <style>
__AGORA_THEME_CSS__
${componentCSS}
    html,
    body {
      background: var(--agora-bg);
      color: var(--agora-fg);
      font: var(--agora-font-status);
      height: 100%;
      margin: 0;
      overflow: hidden;
      width: 100%;
    }
    body {
      box-sizing: border-box;
      display: grid;
      gap: 24px;
      grid-template-rows: auto minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1fr);
      padding: 32px;
    }
    header,
    section {
      max-width: 1120px;
      min-height: 0;
      width: 100%;
    }
    header {
      align-items: center;
      display: flex;
      gap: 18px;
    }
    h1,
    h2 {
      font-size: 20px;
      line-height: 1.2;
      margin: 0;
    }
    h2 {
      font-size: 16px;
      margin-bottom: 10px;
    }
    .mark {
      background: var(--agora-evidence-accent);
      border-radius: var(--agora-radius-control);
      height: 36px;
      width: 36px;
    }
    .overall {
      color: var(--agora-text-muted);
      margin-left: auto;
      padding: 0 2px;
      text-align: right;
    }
    .overall.ok {
      color: var(--agora-accent);
    }
    .overall.warn {
      color: var(--agora-warning);
    }
    .close {
      background: var(--agora-surface-raised);
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      cursor: pointer;
      font: inherit;
      height: var(--agora-control-height);
      min-width: 76px;
      padding: 0 14px;
    }
    .close:hover,
    .close:focus-visible {
      border-color: var(--agora-accent);
    }
    table {
      border-collapse: collapse;
      width: 100%;
    }
    th,
    td {
      border-bottom: 1px solid var(--agora-border-subtle);
      padding: 9px 8px;
      text-align: left;
      vertical-align: top;
    }
    th {
      color: var(--agora-text-muted);
      font-size: 13px;
      text-transform: uppercase;
    }
    code {
      background: var(--agora-surface-raised);
      border: 1px solid var(--agora-border-subtle);
      border-radius: var(--agora-radius-control);
      display: block;
      font: var(--agora-font-code);
      margin: 8px 0;
      overflow-wrap: anywhere;
      padding: 10px;
    }
    .grid {
      display: grid;
      gap: 18px;
      grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
      overflow: hidden;
    }
    .grid > section,
    section[aria-label="Recovery"] {
      overflow: auto;
    }
    .muted {
      color: var(--agora-text-muted);
    }
  </style>
</head>
<body data-surface="operator">
  <header>
    <span class="mark"></span>
    <h1>agora-de shell status</h1>
    <span class="overall warn" id="overall">status: loading</span>
    <button class="close" id="close-button" type="button">OK</button>
  </header>
  <section class="grid" aria-label="Status summaries">
    <section>
      <h2>Services</h2>
      <table>
        <thead><tr><th>Name</th><th>Scope</th><th>State</th></tr></thead>
        <tbody id="services"><tr><td colspan="3">loading</td></tr></tbody>
      </table>
    </section>
    <section>
      <h2>Sockets</h2>
      <table>
        <thead><tr><th>Path</th><th>State</th></tr></thead>
        <tbody id="sockets"><tr><td colspan="2">loading</td></tr></tbody>
      </table>
    </section>
  </section>
  <section class="grid" aria-label="Compositor summaries">
    <section>
      <h2>Outputs</h2>
      <table>
        <thead><tr><th>Name</th><th>State</th><th>Mode</th><th>Size</th></tr></thead>
        <tbody id="outputs"><tr><td colspan="4">loading</td></tr></tbody>
      </table>
    </section>
    <section>
      <h2>Surfaces</h2>
      <table>
        <tbody id="surfaces"><tr><td>loading</td></tr></tbody>
      </table>
    </section>
  </section>
  <section aria-label="Recovery">
    <h2>Recovery</h2>
    <div id="recovery"><code>loading</code></div>
  </section>
  <script>
    ${liveThemeScript}
    function cell(value) {
      const td = document.createElement("td");
      td.textContent = String(value || "");
      return td;
    }

    function renderRows(id, values, mapper, columns) {
      const body = document.getElementById(id);
      body.replaceChildren();
      if (!Array.isArray(values) || values.length === 0) {
        const row = document.createElement("tr");
        const empty = cell("none");
        empty.colSpan = columns;
        row.appendChild(empty);
        body.appendChild(row);
        return;
      }
      values.forEach((value) => body.appendChild(mapper(value)));
    }

    function row(...values) {
      const tr = document.createElement("tr");
      values.forEach((value) => tr.appendChild(cell(value)));
      return tr;
    }

    function renderCommands(target, title, commands) {
      if (!Array.isArray(commands) || commands.length === 0) {
        return;
      }
      const label = document.createElement("div");
      label.className = "muted";
      label.textContent = title;
      target.appendChild(label);
      commands.forEach((command) => {
        const code = document.createElement("code");
        code.textContent = command;
        target.appendChild(code);
      });
    }

    async function refresh() {
      const response = await fetch("/api/operator/status", {cache: "no-store"});
      if (!response.ok) {
        throw new Error("operator status returned " + response.status);
      }
      const status = await response.json();
      const overall = document.getElementById("overall");
      overall.textContent = "status: " + (status.overall || "unknown");
      overall.className = "overall " + (status.overall === "ok" ? "ok" : "warn");

      renderRows("services", status.services, (service) => row(service.name, service.scope, service.state), 3);
      renderRows("sockets", status.sockets, (socket) => row(socket.path, socket.state), 2);
      renderRows("outputs", status.outputs, (output) => {
        const size = output.width && output.height ? output.width + "x" + output.height : output.detail || "";
        return row(output.name, output.state, output.mode || "", size);
      }, 4);

      const surfaces = status.surfaces || {};
      const surfaceBody = document.getElementById("surfaces");
      surfaceBody.replaceChildren(
        row("state", surfaces.state || "unknown"),
        row("total", surfaces.total || 0),
        row("layer shell", surfaces.layerShell || 0),
        row("work", surfaces.work || 0),
        row("focused", surfaces.focused || 0)
      );

      const recovery = status.recovery || {};
      const recoveryTarget = document.getElementById("recovery");
      recoveryTarget.replaceChildren();
      renderCommands(recoveryTarget, "Kill all", [recovery.killAllCommand].filter(Boolean));
      renderCommands(recoveryTarget, "Restart", recovery.restartCommands);
      renderCommands(recoveryTarget, "Live checks", recovery.liveCheckCommands);
      if (recovery.runbook) {
        const runbook = document.createElement("code");
        runbook.textContent = recovery.runbook;
        recoveryTarget.appendChild(runbook);
      }
      if (recovery.note) {
        const note = document.createElement("p");
        note.className = "muted";
        note.textContent = recovery.note;
        recoveryTarget.appendChild(note);
      }
    }

    async function postJSON(path, body) {
      const response = await fetch(path, {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(body)
      });
      if (!response.ok) {
        throw new Error(path + " returned " + response.status);
      }
      return response.json();
    }

    async function closeStatus() {
      try {
        const response = await fetch("/api/surfaces", {cache: "no-store"});
        if (!response.ok) {
          throw new Error("surfaces returned " + response.status);
        }
        const payload = await response.json();
        const surfaces = Array.isArray(payload.surfaces) ? payload.surfaces : [];
        const surface = surfaces.find((candidate) =>
          candidate.mapped && candidate.appId === "io.agorade.ShellStatus"
        ) || surfaces.find((candidate) => candidate.appId === "io.agorade.ShellStatus");
        if (surface && surface.id) {
          await postJSON("/api/surfaces/action", {surfaceId: surface.id, action: "close"});
          return;
        }
      } catch (error) {
        document.getElementById("overall").textContent = "status: close failed";
        document.getElementById("overall").className = "overall warn";
      }
      window.close();
    }

    document.getElementById("close-button").addEventListener("click", closeStatus);
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        closeStatus();
      }
    });
    window.addEventListener("focus", () => refresh().catch(() => {}));
    refresh().catch((error) => {
      document.getElementById("overall").textContent = "status: offline";
      document.getElementById("overall").className = "overall warn";
    });
    setInterval(refresh, 5000);
  </script>
</body>
</html>`;

export const settingsHTML: string = `<!doctype html>
<html>
<head>
  <title>agora-de settings</title>
  <meta name="color-scheme" content="dark">
  <style>
__AGORA_THEME_CSS__
${componentCSS}
    html,
    body {
      background: var(--agora-bg);
      color: var(--agora-fg);
      font: var(--agora-font-status);
      height: 100%;
      margin: 0;
      overflow: hidden;
      width: 100%;
    }
    body {
      box-sizing: border-box;
      display: grid;
      grid-template-rows: auto minmax(0, 1fr) auto;
      min-height: 100vh;
    }
    header,
    footer {
      align-items: center;
      display: flex;
      gap: 12px;
      padding: 16px 18px;
    }
    header {
      border-bottom: 1px solid var(--agora-border-subtle);
    }
    footer {
      border-top: 1px solid var(--agora-border-subtle);
      color: var(--agora-text-muted);
      justify-content: space-between;
      min-height: 48px;
    }
    h1,
    h2 {
      font-size: 20px;
      line-height: 1.2;
      margin: 0;
    }
    h2 {
      font-size: 15px;
    }
    .mark {
      background: var(--agora-evidence-accent);
      border-radius: var(--agora-radius-control);
      height: 30px;
      width: 30px;
    }
    .close {
      background: var(--agora-surface-raised);
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      cursor: pointer;
      font: inherit;
      height: var(--agora-control-height);
      margin-left: auto;
      min-width: 76px;
      padding: 0 14px;
    }
    .settings {
      display: grid;
      gap: 14px;
      min-height: 0;
      overflow-y: auto;
      padding: 18px;
    }
    .setting-row {
      align-items: center;
      background: var(--agora-surface);
      border: 1px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      display: grid;
      gap: 10px;
      grid-template-columns: minmax(0, 1fr) auto;
      min-height: 78px;
      padding: 12px 14px;
    }
    .setting-copy {
      min-width: 0;
    }
    .setting-title,
    .setting-detail {
      display: block;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .setting-title {
      font-weight: 800;
    }
    .setting-detail {
      color: var(--agora-text-muted);
      font-size: 13px;
      margin-top: 5px;
    }
    .toggle {
      align-items: center;
      background: var(--agora-surface-raised);
      border: 2px solid var(--agora-border);
      border-radius: 999px;
      color: var(--agora-fg);
      cursor: pointer;
      display: inline-flex;
      font: inherit;
      height: 36px;
      justify-content: flex-start;
      min-width: 74px;
      padding: 0 4px;
      position: relative;
    }
    .toggle::before {
      background: var(--agora-text-muted);
      border-radius: 999px;
      content: "";
      display: block;
      height: 24px;
      transition: transform 120ms ease, background-color 120ms ease;
      width: 24px;
    }
    .toggle[aria-pressed="true"] {
      border-color: var(--agora-accent);
    }
    .toggle[aria-pressed="true"]::before {
      background: var(--agora-accent);
      transform: translateX(38px);
    }
    .state {
      color: var(--agora-text-muted);
      font: var(--agora-font-code);
    }
    .state.ready {
      color: var(--agora-accent);
    }
    .state.warn {
      color: var(--agora-warning);
    }
  </style>
</head>
<body data-surface="settings">
  <header>
    <span class="mark"></span>
    <h1>Settings</h1>
    <button class="close" id="close-button" type="button" aria-label="Close">${shellIconSVG('close', 'taskbar-button-icon')}<span class="visually-hidden">Close</span></button>
  </header>
  <main class="settings" aria-label="Settings">
    <section class="setting-row">
      <span class="setting-copy">
        <span class="setting-title">Debug overlay</span>
        <span class="setting-detail" id="overlay-detail">loading</span>
      </span>
      <button class="toggle" id="overlay-toggle" type="button" aria-label="Debug overlay" aria-pressed="false"></button>
    </section>
  </main>
  <footer>
    <span id="status">loading</span>
    <span class="state" id="service-state">unknown</span>
  </footer>
  <script>
    ${liveThemeScript}
    const state = {
      settings: null,
      busy: false
    };

    function text(value, fallback) {
      const trimmed = String(value || "").trim();
      return trimmed || fallback;
    }

    async function loadJSON(path) {
      const response = await fetch(path, {cache: "no-store"});
      if (!response.ok) {
        throw new Error(path + " returned " + response.status);
      }
      return response.json();
    }

    async function postJSON(path, body) {
      const response = await fetch(path, {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(body)
      });
      if (!response.ok) {
        let payload = {};
        try {
          payload = await response.json();
        } catch (_error) {
          payload = {};
        }
        throw new Error(text(payload.message, path + " returned " + response.status));
      }
      return response.json();
    }

    function render() {
      const settings = state.settings || {};
      const overlay = settings.service || {};
      const enabled = Boolean(settings.active && settings.active.diagnosticOverlayEnabled);
      const active = Boolean(overlay.active);
      const toggle = document.getElementById("overlay-toggle");
      toggle.setAttribute("aria-pressed", String(enabled));
      toggle.title = enabled ? "Debug overlay on" : "Debug overlay off";
      toggle.disabled = state.busy;
      const detail = document.getElementById("overlay-detail");
      detail.textContent = "user service " + text(overlay.enabledState, "unknown") + " / " + text(overlay.activeState, "unknown");
      const service = document.getElementById("service-state");
      service.textContent = (enabled ? "enabled" : "disabled") + " / " + (active ? "active" : "inactive");
      service.className = "state " + (enabled === active ? "ready" : "warn");
      document.getElementById("status").textContent = state.busy ? "saving" : "ready";
    }

    async function refresh() {
      try {
        state.settings = await loadJSON("/api/settings/modules/diagnostics/load");
        render();
      } catch (error) {
        document.getElementById("status").textContent = "offline";
        document.getElementById("service-state").textContent = "unavailable";
        document.getElementById("service-state").className = "state warn";
      }
    }

    async function toggleOverlay() {
      if (state.busy) {
        return;
      }
      state.busy = true;
      render();
      const enabled = !Boolean(state.settings && state.settings.active && state.settings.active.diagnosticOverlayEnabled);
      try {
        const result = await postJSON("/api/settings/modules/diagnostics/apply", {
          contractVersion: 1,
          baseRevision: Number(state.settings && state.settings.revision || 0),
          draft: {diagnosticOverlayEnabled: enabled}
        });
        state.settings = result.state;
      } catch (error) {
        document.getElementById("status").textContent = "save failed";
      } finally {
        state.busy = false;
        render();
      }
    }

    async function settingsSurface() {
      const surfaces = await loadJSON("/api/surfaces");
      return (Array.isArray(surfaces.surfaces) ? surfaces.surfaces : []).find((surface) =>
        surface.mapped && surface.appId === "io.agorade.ShellSettings"
      );
    }

    async function closeSettings() {
      try {
        const surface = await settingsSurface();
        if (surface) {
          await postJSON("/api/surfaces/action", {surfaceId: surface.id, action: "close"});
        } else {
          window.close();
        }
      } catch (error) {
        window.close();
      }
    }

    document.getElementById("overlay-toggle").addEventListener("click", toggleOverlay);
    document.getElementById("close-button").addEventListener("click", closeSettings);
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        closeSettings();
      }
    });
    refresh();
    setInterval(refresh, 5000);
  </script>
</body>
</html>`;

export const launcherHTML: string = `<!doctype html>
<html>
<head>
  <title>agora-de app launcher</title>
  <meta name="color-scheme" content="dark">
  <style>
__AGORA_THEME_CSS__
${componentCSS}
    html,
    body {
      background: var(--agora-bg);
      color: var(--agora-fg);
      font: var(--agora-font-status);
      height: 100%;
      margin: 0;
      overflow: hidden;
      width: 100%;
    }
    body {
      box-sizing: border-box;
      display: block;
      height: 100vh;
      min-height: 0;
    }
    .launcher {
      background: var(--agora-surface);
      border: 2px solid var(--agora-border);
      border-bottom-color: var(--agora-accent);
      border-radius: var(--agora-radius-control);
      box-shadow: var(--agora-popup-shadow);
      display: grid;
      grid-template-rows: auto 1fr auto;
      height: 100vh;
      inset: 0;
      min-height: 0;
      overflow: hidden;
      position: fixed;
      width: 100vw;
    }
    .launcher-header {
      align-items: center;
      border-bottom: 1px solid var(--agora-border-subtle);
      display: flex;
      gap: 12px;
      padding: 14px;
    }
    .mark {
      background: var(--agora-evidence-accent);
      border-radius: var(--agora-radius-control);
      height: 28px;
      width: 28px;
    }
    .title {
      font-weight: 700;
      min-width: 120px;
    }
    .search {
      background: var(--agora-surface-raised);
      border: 1px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      flex: 1 1 auto;
      font: inherit;
      height: var(--agora-control-height);
      min-width: 180px;
      padding: 0 12px;
    }
    .close {
      background: var(--agora-surface-raised);
      border: 1px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      font: inherit;
      height: var(--agora-control-height);
      min-width: 72px;
    }
    .launcher-body {
      display: grid;
      grid-template-columns: 176px minmax(0, 1fr);
      min-height: 0;
      overflow: hidden;
    }
    .categories {
      background: var(--agora-evidence-strong);
      border-right: 1px solid var(--agora-border-subtle);
      display: flex;
      flex-direction: column;
      gap: 6px;
      overflow-y: auto;
      padding: 12px;
    }
    .category {
      background: transparent;
      border: 1px solid transparent;
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      font: inherit;
      min-height: 36px;
      padding: 0 10px;
      text-align: left;
    }
    .category.active {
      background: var(--agora-surface-raised);
      border-color: var(--agora-accent);
    }
    .apps {
      display: grid;
      grid-template-rows: auto 1fr;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }
    .summary {
      border-bottom: 1px solid var(--agora-border-subtle);
      color: var(--agora-text-muted);
      font-size: 13px;
      padding: 10px 14px;
    }
    .app-list {
      display: flex;
      flex-direction: column;
      gap: 8px;
      min-height: 0;
      overflow-y: auto;
      padding: 12px;
    }
    .app {
      align-items: center;
      background: var(--agora-surface-raised);
      border: 1px solid var(--agora-border-subtle);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      display: grid;
      gap: 10px;
      grid-template-columns: 34px minmax(0, 1fr);
      min-height: 54px;
      padding: 8px 10px;
      text-align: left;
    }
    .app:disabled {
      opacity: 0.72;
    }
    .app:not(:disabled) {
      cursor: pointer;
    }
    .app:not(:disabled):hover,
    .app:not(:disabled):focus-visible {
      border-color: var(--agora-accent);
    }
    .app-icon {
      align-items: center;
      background: var(--agora-evidence-strong);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      display: inline-flex;
      font-size: 14px;
      height: 34px;
      justify-content: center;
      overflow: hidden;
      width: 34px;
    }
    .app-icon img {
      display: block;
      height: 100%;
      object-fit: contain;
      width: 100%;
    }
    .app-icon.has-image .icon-fallback {
      display: none;
    }
    .app-icon.icon-load-failed img {
      display: none;
    }
    .app-icon.icon-load-failed .icon-fallback {
      display: inline;
    }
    .app-name,
    .app-detail {
      display: block;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .app-detail {
      color: var(--agora-text-muted);
      font-size: 12px;
      margin-top: 3px;
    }
    .footer {
      align-items: center;
      border-top: 1px solid var(--agora-border-subtle);
      color: var(--agora-text-muted);
      display: flex;
      font-size: 13px;
      justify-content: space-between;
      min-height: 42px;
      padding: 0 14px;
    }
  </style>
</head>
<body data-surface="launcher">
  <main class="launcher" aria-label="Agora DE app launcher">
    <header class="launcher-header">
      <span class="mark"></span>
      <span class="title">Applications</span>
      <input class="search" id="app-search" type="search" aria-label="Search apps" placeholder="Search">
      <button class="close" id="close-button" type="button" aria-label="Close">${shellIconSVG('close', 'taskbar-button-icon')}<span class="visually-hidden">Close</span></button>
    </header>
    <section class="launcher-body">
      <nav class="categories" id="categories" aria-label="Application categories"></nav>
      <section class="apps" aria-label="Applications">
        <div class="summary" id="summary">loading apps</div>
        <div class="app-list" id="app-list"></div>
      </section>
    </section>
    <footer class="footer">
      <span id="status">loading</span>
      <span id="policy-status">checking apps</span>
    </footer>
  </main>
  <script>
    ${liveThemeScript}
    const state = {
      apps: [],
      query: "",
      category: "All"
    };

    function text(value, fallback) {
      const trimmed = String(value || "").trim();
      return trimmed || fallback;
    }

    function categories() {
      const names = new Set(["All"]);
      state.apps.forEach((app) => names.add(text(app.category, "Other")));
      return Array.from(names).sort((left, right) => left === "All" ? -1 : right === "All" ? 1 : left.localeCompare(right));
    }

    function filteredApps() {
      const query = state.query.trim().toLowerCase();
      return state.apps.filter((app) => {
        const category = text(app.category, "Other");
        if (state.category !== "All" && category !== state.category) {
          return false;
        }
        if (!query) {
          return true;
        }
        const haystack = [
          text(app.name, app.id),
          text(app.id, ""),
          category,
          ...(Array.isArray(app.categories) ? app.categories : [])
        ].join(" ").toLowerCase();
        return haystack.includes(query);
      });
    }

    function createIcon(className, label, iconUrl, title) {
      const icon = document.createElement("span");
      icon.className = className;
      icon.title = title || "";
      const fallback = document.createElement("span");
      fallback.className = "icon-fallback";
      fallback.innerHTML = '${appFallbackIconSVG}';
      if (iconUrl) {
        icon.classList.add("has-image");
        const image = document.createElement("img");
        image.src = iconUrl;
        image.alt = "";
        image.decoding = "async";
        image.loading = "lazy";
        image.addEventListener("error", () => icon.classList.add("icon-load-failed"), {once: true});
        icon.appendChild(image);
      }
      icon.appendChild(fallback);
      return icon;
    }

    function renderCategories() {
      const target = document.getElementById("categories");
      target.replaceChildren();
      categories().forEach((category) => {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "category" + (category === state.category ? " active" : "");
        button.textContent = category;
        button.addEventListener("click", () => {
          state.category = category;
          render();
        });
        target.appendChild(button);
      });
    }

    function renderApp(app) {
      const label = text(app.name, app.id);
      const reason = text(app.disabledReason, app.launchable ? "" : "not launchable");
      const button = document.createElement("button");
      button.type = "button";
      button.className = "app";
      button.disabled = !app.launchable;
      button.dataset.appId = app.id;
      button.dataset.disabledCode = text(app.disabledCode, "");
      button.setAttribute("aria-disabled", String(!app.launchable));
      button.title = reason ? label + " - " + reason : label;
      button.addEventListener("click", () => launchApp(app.id));

      const icon = createIcon(
        "app-icon",
        text(app.iconLabel, label.slice(0, 1).toUpperCase()),
        text(app.iconUrl, ""),
        text(app.iconRef, text(app.icon, ""))
      );
      const copy = document.createElement("span");
      const name = document.createElement("span");
      name.className = "app-name";
      name.textContent = label;
      const detail = document.createElement("span");
      detail.className = "app-detail";
      detail.textContent = reason ? reason : text(app.category, "Other");
      copy.appendChild(name);
      copy.appendChild(detail);
      button.appendChild(icon);
      button.appendChild(copy);
      return button;
    }

    function render() {
      renderCategories();
      const apps = filteredApps();
      const list = document.getElementById("app-list");
      list.replaceChildren();
      if (!apps.length) {
        const empty = document.createElement("div");
        empty.className = "app-detail";
        empty.textContent = "no matching apps";
        list.appendChild(empty);
      } else {
        apps.forEach((app) => list.appendChild(renderApp(app)));
      }
      document.getElementById("summary").textContent = apps.length + " of " + state.apps.length + " apps";
      document.getElementById("status").textContent = state.category + (state.query ? " search" : "");
      const launchable = state.apps.filter((app) => app.launchable === true).length;
      const disabled = state.apps.length - launchable;
      document.getElementById("policy-status").textContent = launchable + " launchable / " + disabled + " disabled";
    }

    async function loadJSON(path) {
      const response = await fetch(path, {cache: "no-store"});
      if (!response.ok) {
        throw new Error(path + " returned " + response.status);
      }
      return response.json();
    }

    async function postJSON(path, body) {
      const response = await fetch(path, {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(body)
      });
      if (!response.ok) {
        let errorBody = {};
        try {
          errorBody = await response.json();
        } catch (_error) {
          errorBody = {};
        }
        const error = new Error(text(errorBody.error, path + " returned " + response.status));
        error.status = response.status;
        error.errorClass = text(errorBody.errorClass, "");
        throw error;
      }
      return response.json();
    }

    async function refresh() {
      try {
        const catalog = await loadJSON("/api/catalog/apps");
        state.apps = Array.isArray(catalog.apps) ? catalog.apps : [];
        render();
      } catch (error) {
        document.getElementById("summary").textContent = "catalog offline";
        document.getElementById("status").textContent = "offline";
      }
    }

    async function launcherSurface() {
      const surfaces = await loadJSON("/api/surfaces");
      return (Array.isArray(surfaces.surfaces) ? surfaces.surfaces : []).find((surface) =>
        surface.mapped && surface.appId === "io.agorade.ShellLauncher"
      );
    }

    async function closeLauncher() {
      try {
        const surface = await launcherSurface();
        if (surface) {
          await postJSON("/api/surfaces/action", {surfaceId: surface.id, action: "close"});
        } else {
          window.close();
        }
      } catch (error) {
        window.close();
      }
    }

    async function launchApp(appId) {
      document.getElementById("status").textContent = "launching";
      try {
        await postJSON("/api/catalog/launch", {appId});
        document.getElementById("status").textContent = "launch accepted";
        await closeLauncher();
      } catch (error) {
        document.getElementById("status").textContent = "launch failed";
      }
    }

    document.getElementById("app-search").addEventListener("input", (event) => {
      state.query = event.target.value;
      render();
    });
    document.getElementById("close-button").addEventListener("click", closeLauncher);
    document.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        closeLauncher();
      }
    });
    window.addEventListener("focus", refresh);
    refresh().then(() => document.getElementById("app-search").focus());
  </script>
</body>
</html>`;

export const panelHTML: string = `<!doctype html>
<html>
<head>
  <title>agora-de shell panel</title>
  <meta name="color-scheme" content="dark">
  <style>
__AGORA_THEME_CSS__
${componentCSS}
    html,
    body {
      background: transparent !important;
      color: var(--agora-fg);
      height: 100%;
      margin: 0;
      overflow: hidden;
      width: 100%;
    }
    body {
      align-items: stretch;
      background: transparent;
      box-sizing: border-box;
      display: flex;
      font: 600 14px var(--agora-font-family);
      overflow: hidden;
    }
    .panel {
      --taskbar-control-height: var(--agora-panel-control-height);
      align-items: center;
      background: var(--agora-panel-bg);
      border-top: 3px solid var(--agora-accent);
      bottom: 0;
      box-shadow: var(--agora-panel-shadow);
      box-sizing: border-box;
      display: grid;
      gap: 8px;
      grid-template-columns: auto minmax(220px, 1fr) auto auto auto auto;
      height: var(--agora-panel-height);
      left: 0;
      min-height: var(--agora-panel-height);
      padding: var(--agora-panel-padding-y) var(--agora-panel-padding-x);
      position: fixed;
      right: 0;
      width: 100vw;
    }
    .taskbar-start,
    .taskbar-tray,
    .workspace-strip {
      align-items: center;
      display: inline-flex;
      gap: 6px;
      min-width: 0;
    }
    .start-button,
    .taskbar-icon-button,
    .workspace,
    .layout-status,
    .status,
    .clock {
      align-items: center;
      border-radius: var(--agora-radius-control);
      display: inline-flex;
      box-sizing: border-box;
      height: var(--taskbar-control-height);
      justify-content: center;
      min-width: 44px;
      padding: 0 10px;
      white-space: nowrap;
    }
    .start-button {
      background: var(--agora-evidence-strong);
      border: 2px solid var(--agora-evidence-accent);
      color: var(--agora-fg);
      cursor: pointer;
      font-weight: 800;
      min-width: 86px;
    }
    .start-button[aria-pressed="true"] {
      background: var(--agora-accent);
      color: var(--agora-bg);
    }
    .taskbar-icon-button {
      background: var(--agora-surface-raised);
      border: 2px solid var(--agora-border);
      color: var(--agora-fg);
      cursor: pointer;
      min-width: var(--taskbar-control-height);
      padding: 0;
      width: var(--taskbar-control-height);
    }
    .taskbar-button-icon {
      display: block;
      height: 18px;
      pointer-events: none;
      width: 18px;
    }
    .taskbar-icon-button:hover,
    .workspace:hover,
    .layout-status:hover,
    .wm-menu summary:hover,
    .task-button:hover {
      border-color: var(--agora-accent);
    }
    .workspace,
    .layout-status,
    .status,
    .clock {
      border: 2px solid var(--agora-border);
      color: var(--agora-fg);
    }
    .workspace {
      background: var(--agora-surface-raised);
      cursor: pointer;
      font: inherit;
      gap: 5px;
      min-width: 42px;
      position: relative;
    }
    .workspace.active {
      background: var(--agora-surface-strong);
      border-color: var(--agora-accent);
      box-shadow: inset 0 -4px 0 var(--agora-accent);
    }
    .workspace-count {
      align-items: center;
      background: var(--agora-accent);
      border-radius: 999px;
      color: var(--agora-bg);
      display: inline-flex;
      font: 800 12px var(--agora-font-family);
      height: 16px;
      justify-content: center;
      margin-left: 5px;
      min-width: 16px;
      padding: 0 4px;
    }
    .workspace-output {
      color: var(--agora-text-muted);
      font: 700 10px var(--agora-font-family);
      text-transform: uppercase;
    }
    .layout-status {
      background: var(--agora-surface-raised);
      cursor: pointer;
      font: inherit;
      max-width: 116px;
      min-width: 92px;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .taskbar-tasks {
      align-items: center;
      display: flex;
      gap: 6px;
      min-width: 0;
      overflow-x: auto;
      overflow-y: hidden;
      scrollbar-width: thin;
    }
    .taskbar-tasks::-webkit-scrollbar {
      height: 5px;
    }
    .taskbar-tasks::-webkit-scrollbar-thumb {
      background: var(--agora-border-subtle);
    }
    .task-button,
    .dock-item {
      align-items: center;
      background: var(--agora-surface-raised);
      border: 2px solid var(--agora-border-subtle);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      display: inline-flex;
      gap: 8px;
      height: var(--taskbar-control-height);
      min-width: 0;
      overflow: hidden;
      padding: 0 10px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .task-button {
      cursor: pointer;
      flex: 1 1 168px;
      max-width: 230px;
      min-width: 104px;
      position: relative;
    }
    .task-button.focused {
      background: var(--agora-surface-strong);
      border-color: var(--agora-accent);
      box-shadow: inset 0 -3px 0 var(--agora-accent);
    }
    .task-button.pinned {
      box-shadow: inset 0 -3px 0 var(--agora-taskbar-pin, var(--agora-accent-soft, rgba(94, 196, 168, 0.5)));
    }
    .task-button.pinned.focused {
      box-shadow: inset 0 -3px 0 var(--agora-accent);
    }
    .task-button.launcher {
      opacity: 0.85;
    }
    .task-button.launcher:hover {
      opacity: 1;
    }
    .wm-select {
      background: var(--agora-surface-raised);
      border: 1px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      color: var(--agora-text);
      padding: 0 8px;
      height: var(--taskbar-control-height, 32px);
      min-width: 120px;
      font: inherit;
    }
    .task-button.minimized {
      background: var(--agora-taskbar-minimized-bg);
      border-color: var(--agora-taskbar-minimized-border);
      color: var(--agora-text-muted);
    }
    .task-button.minimized .task-label::after {
      border: 1px solid var(--agora-warning);
      border-radius: var(--agora-radius-control);
      color: var(--agora-warning);
      content: "min";
      font: 800 10px var(--agora-font-family);
      margin-left: 7px;
      padding: 1px 4px;
      text-transform: uppercase;
    }
    .task-icon {
      align-items: center;
      background: var(--agora-evidence-strong);
      border: 1px solid var(--agora-border-subtle);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      display: inline-flex;
      flex: 0 0 28px;
      font: 800 14px var(--agora-font-family);
      height: 26px;
      justify-content: center;
      overflow: hidden;
      width: 26px;
    }
    .task-icon img {
      display: block;
      height: 100%;
      object-fit: contain;
      width: 100%;
    }
    .task-icon.has-image .icon-fallback {
      display: none;
    }
    .task-icon.icon-load-failed img {
      display: none;
    }
    .task-icon.icon-load-failed .icon-fallback {
      display: inline;
    }
    .task-label {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    button.dock-item {
      cursor: pointer;
    }
    .dock-item.muted {
      flex: 0 0 auto;
      min-width: 132px;
    }
    .surface-actions {
      align-items: center;
      display: inline-flex;
      gap: 6px;
      min-width: 0;
    }
    .surface-action {
      background: var(--agora-surface-raised);
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      height: var(--taskbar-control-height);
      min-width: 58px;
      padding: 0 10px;
    }
    .wm-menu {
      position: relative;
    }
    .wm-menu summary {
      align-items: center;
      background: var(--agora-surface-raised);
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      box-sizing: border-box;
      color: var(--agora-fg);
      cursor: pointer;
      display: inline-flex;
      height: var(--taskbar-control-height);
      justify-content: center;
      list-style: none;
      min-width: 54px;
      padding: 0 12px;
      white-space: nowrap;
    }
    .wm-menu summary::-webkit-details-marker {
      display: none;
    }
    .wm-controls {
      align-items: center;
      background: var(--agora-surface);
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      bottom: calc(var(--agora-panel-height) - 2px);
      box-shadow: var(--agora-popup-shadow);
      display: none;
      gap: 8px;
      padding: 10px;
      position: absolute;
      right: 0;
      width: min(760px, calc(100vw - 44px));
      z-index: 5;
    }
    .wm-menu[open] .wm-controls {
      display: flex;
      flex-wrap: wrap;
    }
    .wm-control {
      background: var(--agora-surface);
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      flex: 0 0 auto;
      height: var(--taskbar-control-height);
      min-width: 54px;
      padding: 0 10px;
    }
    .wm-control.primary {
      border-color: var(--agora-accent);
    }
    .wm-control:disabled {
      color: var(--agora-text-muted);
      cursor: default;
      opacity: 0.65;
    }
    .wm-rule {
      background: var(--agora-surface-strong);
      border-color: var(--agora-border-subtle);
      max-width: 150px;
      min-width: 96px;
    }
    .surface-meta {
      font-size: 12px;
      min-width: 78px;
      padding: 0 8px;
    }
    .target-meta {
      border-color: var(--agora-accent);
      max-width: 170px;
      min-width: 104px;
    }
    .status {
      background: var(--agora-surface-strong);
      max-width: 126px;
      min-width: 92px;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .clock {
      min-width: 68px;
      width: 68px;
    }
    .status.ready {
      border-color: var(--agora-accent);
    }
    .status.warn {
      border-color: var(--agora-warning);
    }
    .muted {
      color: var(--agora-text-muted);
    }
    @media (max-width: 980px) {
      .panel {
        gap: 8px;
        grid-template-columns: auto minmax(120px, 1fr) auto auto;
        padding-inline: 10px;
      }
      .layout-status,
      .status {
        display: none;
      }
      .taskbar-icon-button {
        min-width: var(--taskbar-control-height);
        padding: 0;
      }
      .start-button {
        min-width: 72px;
      }
    }
  </style>
</head>
<body data-surface="__AGORA_SURFACE__">
  <main class="panel taskbar" aria-label="Agora DE shell panel">
    <section class="taskbar-start" aria-label="Shell controls">
      <button class="start-button" id="apps-button" type="button" aria-pressed="false">Start</button>
      <button class="taskbar-icon-button" id="refresh-button" type="button" aria-label="Refresh" title="Refresh">
        ${shellIconSVG('refresh', 'taskbar-button-icon')}
        <span class="visually-hidden">Refresh</span>
      </button>
      <button class="taskbar-icon-button" id="operator-button" type="button" aria-label="Status" title="Status">
        ${shellIconSVG('status', 'taskbar-button-icon')}
        <span class="visually-hidden">Status</span>
      </button>
      <button class="taskbar-icon-button" id="settings-button" type="button" aria-label="Settings" title="Settings">
        ${shellIconSVG('settings', 'taskbar-button-icon')}
        <span class="visually-hidden">Settings</span>
      </button>
    </section>
    <section class="taskbar-tasks running" id="running-list" aria-label="Running surfaces">
      <span class="dock-item muted">loading surfaces</span>
    </section>
    <details class="wm-menu" id="wm-menu">
      <summary>WM</summary>
      <section class="wm-controls" id="wm-controls" aria-label="Window controls">
        <span class="dock-item surface-meta target-meta" id="target-label">no target</span>
        <button class="wm-control" id="focus-prev-button" type="button">Prev</button>
        <button class="wm-control" id="focus-next-button" type="button">Next</button>
        <button class="wm-control primary" id="promote-button" type="button">Master</button>
        <button class="wm-control" id="move-left-button" type="button" title="Move focused window left (toward master)">◀</button>
        <button class="wm-control" id="move-right-button" type="button" title="Move focused window right (toward stack)">▶</button>
        <button class="wm-control" id="move-up-button" type="button" title="Move focused window up">▲</button>
        <button class="wm-control" id="move-down-button" type="button" title="Move focused window down">▼</button>
        <button class="wm-control" id="swap-master-button" type="button" title="Swap focused window with the master">Swap</button>
        <button class="wm-control" id="move-zone-button" type="button">Move zone</button>
        <button class="wm-control" id="float-button" type="button">Float</button>
        <button class="wm-control" id="fullscreen-button" type="button">Full</button>
        <button class="wm-control" id="maximize-button" type="button">Max</button>
        <button class="wm-control" id="minimize-button" type="button">Min</button>
        <button class="wm-control" id="close-focus-button" type="button">Close</button>
        <button class="wm-control" id="reset-layout-button" type="button">Reset</button>
        <span class="dock-item surface-meta wm-rule" id="layout-rule-label">master_stack</span>
      </section>
    </details>
    <section class="workspace-strip" id="workspace-list" aria-label="Workspaces">
      <button class="workspace active" id="workspace-label" type="button" data-workspace-id="workspace-1">1</button>
    </section>
    <button class="layout-status" id="layout-mode-button" type="button">freeform</button>
    <section class="taskbar-tray" aria-label="Status tray">
      <span class="status" id="status-label">starting</span>
      <time class="clock" id="clock-label">--:--</time>
    </section>
  </main>
  <script>
    ${liveThemeScript}
    const state = {
      apps: [],
      surfaces: [],
      pins: [],
      layout: {mode: "freeform", revision: 0, surfaces: [], workspaces: []},
      workspaceState: {currentWorkspaceId: "workspace-1", currentOutputId: "", workspaces: []},
      workspace: {id: "workspace-1", name: "workspace 1", active: true, surfaceCount: 0},
      feedback: {label: "", className: "", until: 0},
      surface: "__AGORA_SURFACE__"
    };
    const unsupportedSurfaceActions = new Set([]);

    function text(value, fallback) {
      const trimmed = String(value || "").trim();
      return trimmed || fallback;
    }

    function item(label, className) {
      const element = document.createElement("span");
      element.className = "dock-item" + (className ? " " + className : "");
      element.textContent = label;
      element.title = label;
      return element;
    }

    function button(label, className, onClick) {
      const element = document.createElement("button");
      element.type = "button";
      element.className = className;
      element.textContent = label;
      element.title = label;
      element.addEventListener("click", onClick);
      return element;
    }

    function renderList(id, emptyLabel, values, mapper, limit) {
      const target = document.getElementById(id);
      target.replaceChildren();
      if (!values.length) {
        target.appendChild(item(emptyLabel, "muted"));
        return;
      }
      values.slice(0, limit || 4).forEach((value) => target.appendChild(mapper(value)));
    }

    function launcherSurface() {
      return state.surfaces.find((surface) =>
        surface.mapped && surface.appId === "io.agorade.ShellLauncher"
      );
    }

    function layoutSurface(surfaceId) {
      const surfaces = Array.isArray(state.layout.surfaces) ? state.layout.surfaces : [];
      return surfaces.find((surface) => surface.surfaceId === surfaceId);
    }

    function layoutSurfaces() {
      return Array.isArray(state.layout.surfaces) ? state.layout.surfaces : [];
    }

    function manageableLayoutSurfaces() {
      return layoutSurfaces()
        .filter((surface) => surface.visible !== false && surface.participation !== "transient")
        .sort((left, right) => Number(left.order || 0) - Number(right.order || 0));
    }

    function focusedLayoutSurface() {
      return manageableLayoutSurfaces().find((surface) => surface.focused) || manageableLayoutSurfaces()[0] || null;
    }

    function targetSurface() {
      const layout = focusedLayoutSurface();
      if (layout) {
        const surface = state.surfaces.find((candidate) => candidate.id === layout.surfaceId) || {};
        return {
          ...layout,
          fullscreen: Boolean(surface.fullscreen),
          maximized: Boolean(surface.maximized),
          minimized: Boolean(surface.minimized)
        };
      }
      const surface = state.surfaces.find((candidate) => isTaskbarWorkSurface(candidate) && candidate.focused) ||
        state.surfaces.find((candidate) => isTaskbarWorkSurface(candidate));
      return surface ? {
        surfaceId: surface.id,
        appId: surface.appId,
        title: surface.title,
        zoneId: surface.zoneId,
        focused: surface.focused,
        floating: surface.layoutRole === "floating",
        fullscreen: Boolean(surface.fullscreen),
        maximized: Boolean(surface.maximized),
        minimized: Boolean(surface.minimized)
      } : null;
    }

    function activeLayoutWorkspace() {
      const workspaces = Array.isArray(state.layout.workspaces) ? state.layout.workspaces : [];
      const currentId = text(state.workspaceState.currentWorkspaceId, "");
      return (currentId ? workspaces.find((workspace) => workspace.id === currentId) : null) ||
        workspaces.find((workspace) => workspace.active) ||
        workspaces.find((workspace) => workspace.id === state.workspace.id) ||
        workspaces[0] ||
        {zones: []};
    }

    function workspaceNumber(workspaceId) {
      const match = String(workspaceId || "").match(/^workspace-(\d+)$/);
      return match ? Number(match[1]) : 0;
    }

    function allWorkspaces() {
      const fromLayout = Array.isArray(state.layout.workspaces) ? state.layout.workspaces : [];
      const fromWorkspaceState = Array.isArray(state.workspaceState.workspaces) ? state.workspaceState.workspaces : [];
      const fromState = state.workspace && state.workspace.id ? [state.workspace] : [];
      const byId = new Map();
      fromLayout.concat(fromWorkspaceState, fromState).forEach((workspace) => {
        const id = text(workspace && workspace.id, "");
        if (!id || byId.has(id)) {
          return;
        }
        byId.set(id, workspace);
      });
      if (!byId.size) {
        byId.set("workspace-1", {id: "workspace-1", name: "workspace 1", active: true, surfaceCount: 0});
      }
      return Array.from(byId.values()).sort((left, right) => workspaceNumber(left.id) - workspaceNumber(right.id));
    }

    function activeWorkspaceId() {
      return text(state.workspaceState.currentWorkspaceId, text(state.workspace.id, text(activeLayoutWorkspace().id, "workspace-1")));
    }

    function workspaceById(workspaceId) {
      return allWorkspaces().find((workspace) => text(workspace.id, "") === workspaceId) || null;
    }

    function workspaceOutputId(workspace) {
      return text(workspace && workspace.outputId, "");
    }

    function workspaceOutputShortName(outputId) {
      const value = text(outputId, "");
      if (!value) {
        return "";
      }
      const parts = value.split("-");
      if (parts.length >= 3) {
        return parts[0][0] + parts[1][0] + parts[parts.length - 1];
      }
      return value.length > 5 ? value.slice(0, 5) : value;
    }

    function workspaceShortName(workspace) {
      const id = text(workspace && workspace.id, "workspace-1");
      const number = workspaceNumber(id);
      return number ? String(number) : text(workspace && workspace.name, id);
    }

    function workspaceSurfaceCount(workspace) {
      if (Number.isFinite(Number(workspace && workspace.surfaceCount))) {
        return Number(workspace.surfaceCount);
      }
      const order = Array.isArray(workspace && workspace.surfaceOrder) ? workspace.surfaceOrder : [];
      return order.length;
    }

    function nextWorkspaceId() {
      const workspaces = allWorkspaces();
      const ids = workspaces.map((workspace) => text(workspace.id, "")).filter(Boolean);
      const activeId = activeWorkspaceId();
      if (ids.length <= 1) {
        const nextNumber = Math.max(2, workspaceNumber(activeId) + 1);
        return "workspace-" + nextNumber;
      }
      const index = ids.indexOf(activeId);
      return ids[(Math.max(index, 0) + 1) % ids.length];
    }

    function surfaceWorkspaceId(surface) {
      const layout = layoutSurface(surface.id) || {};
      return text(layout.workspaceId, text(surface.workspaceId, activeWorkspaceId()));
    }

    function lowerText(value) {
      return text(value, "").toLowerCase();
    }

    function createIcon(className, label, iconUrl, title) {
      const icon = document.createElement("span");
      icon.className = className;
      icon.title = title || "";
      const fallback = document.createElement("span");
      fallback.className = "icon-fallback";
      fallback.innerHTML = '${appFallbackIconSVG}';
      if (iconUrl) {
        icon.classList.add("has-image");
        const image = document.createElement("img");
        image.src = iconUrl;
        image.alt = "";
        image.decoding = "async";
        image.loading = "lazy";
        image.addEventListener("error", () => icon.classList.add("icon-load-failed"), {once: true});
        icon.appendChild(image);
      }
      icon.appendChild(fallback);
      return icon;
    }

    function isShellManagedSurface(surface) {
      const appId = text(surface && surface.appId, "");
      return appId.indexOf("io.agorade.Shell") === 0;
    }

    function isTransientSurfaceRole(value) {
      const role = lowerText(value);
      return ["dialog", "modal", "popup", "popover", "menu", "tooltip", "transient", "unmanaged"]
        .some((marker) => role === marker || role.indexOf(marker) >= 0);
    }

    function surfacePolicyClass(surface, layout) {
      return lowerText(text(layout && layout.policyClass, text(surface && surface.policyClass, "")));
    }

    function isTaskbarTransientPolicy(policyClass) {
      return ["transient", "shell_chrome", "no_parent", "stale", "unsupported"]
        .some((policy) => policy === policyClass);
    }

    function isTaskbarWorkSurface(surface) {
      if (!surface || !surface.mapped) {
        return false;
      }
      const layout = layoutSurface(surface.id) || {};
      if (isTaskbarTransientPolicy(surfacePolicyClass(surface, layout))) {
        return false;
      }
      if (surface.surfaceKind === "layer_shell" || isShellManagedSurface(surface)) {
        return false;
      }
      if (isTransientSurfaceRole(surface.role) ||
        isTransientSurfaceRole(surface.layoutRole) ||
        isTransientSurfaceRole(layout.role) ||
        isTransientSurfaceRole(layout.participation)) {
        return false;
      }
      const zone = lowerText(text(layout.zoneId, surface.zoneId));
      if (zone === "chrome" || zone === "transient") {
        return false;
      }
      return surfaceWorkspaceId(surface) === activeWorkspaceId();
    }

    function surfacePolicyLabel(surface, layout) {
      const policy = text(layout && layout.policyClass, text(surface && surface.policyClass, "work"));
      const reason = text(layout && layout.policyReason, text(surface && surface.policyReason, ""));
      return reason ? policy + " / " + reason : policy;
    }

    function appIconLabel(surface, layout) {
      const app = catalogAppForSurface(surface, layout);
      if (app) {
        return text(app.iconLabel, text(app.name, text(app.id, "?")).slice(0, 1).toUpperCase());
      }
      const source = text(surface.appId, text(layout.appId, text(surface.title, surface.id)));
      return source.slice(0, 1).toUpperCase();
    }

    function normalizeAppKey(value) {
      return text(value, "").toLowerCase().replace(/\.desktop$/, "");
    }

    function catalogAppForSurface(surface, layout) {
      const keys = new Set([
        normalizeAppKey(surface && surface.appId),
        normalizeAppKey(layout && layout.appId),
        normalizeAppKey(surface && surface.id),
        normalizeAppKey(surface && surface.title)
      ]);
      return state.apps.find((app) => {
        const appKeys = [
          normalizeAppKey(app.id),
          normalizeAppKey(app.name),
          normalizeAppKey(app.startupWMClass),
          normalizeAppKey(app.iconRef),
          normalizeAppKey(app.icon)
        ];
        return appKeys.some((key) => key && keys.has(key));
      }) || null;
    }

    function appIconURL(surface, layout) {
      const app = catalogAppForSurface(surface, layout);
      return app ? text(app.iconUrl, "") : "";
    }

    function catalogAppById(appId) {
      const key = normalizeAppKey(appId);
      if (!key) {
        return null;
      }
      return state.apps.find((app) =>
        [normalizeAppKey(app.id), normalizeAppKey(app.name), normalizeAppKey(app.startupWMClass)]
          .some((candidate) => candidate === key)
      ) || null;
    }

    function taskbarAppId(surface) {
      const layout = layoutSurface(surface.id) || {};
      return text(surface.appId, text(layout.appId, text(surface.title, surface.id)));
    }

    // Build the merged taskbar list: pinned apps first (each carrying its running
    // surface if one matches), then running surfaces not matched to a pin.
    function taskbarEntries(workSurfaces) {
      const pins = Array.isArray(state.pins) ? state.pins : [];
      const used = new Set();
      const entries = [];
      pins.forEach((appId) => {
        const surface = workSurfaces.find((candidate) => {
          if (used.has(candidate.surfaceId || candidate.id)) {
            return false;
          }
          return normalizeAppKey(taskbarAppId(candidate)) === normalizeAppKey(appId);
        });
        if (surface) {
          used.add(surface.surfaceId || surface.id);
        }
        entries.push({appId, pinned: true, surface: surface || null});
      });
      workSurfaces.forEach((surface) => {
        const id = surface.surfaceId || surface.id;
        if (!used.has(id)) {
          entries.push({appId: taskbarAppId(surface), pinned: false, surface});
        }
      });
      return entries;
    }

    function renderTaskbar(workSurfaces) {
      const target = document.getElementById("running-list");
      target.replaceChildren();
      const entries = taskbarEntries(workSurfaces);
      if (!entries.length) {
        const empty = document.createElement("span");
        empty.className = "dock-item muted";
        empty.textContent = "no apps";
        target.appendChild(empty);
        return;
      }
      entries.forEach((entry) => {
        const surface = entry.surface;
        const layout = surface ? (layoutSurface(surface.id) || {}) : {};
        const app = catalogAppById(entry.appId);
        const iconLabel = surface
          ? appIconLabel(surface, layout)
          : text(text(app && app.name, entry.appId).slice(0, 1).toUpperCase(), "?");
        const iconUrl = surface ? appIconURL(surface, layout) : text(app && app.iconUrl, "");
        const taskLabel = surface
          ? text(surface.title, text(surface.appId, surface.id))
          : text(app && app.name, entry.appId);
        const focused = Boolean(surface && (surface.focused || layout.focused));
        const minimized = Boolean(surface && surface.minimized);
        const focusButton = button(
          "",
          "task-button" +
            (focused ? " focused" : "") +
            (minimized ? " minimized" : "") +
            (entry.pinned ? " pinned" : "") +
            (!surface ? " launcher" : ""),
          () => (surface ? activateTaskSurface(surface) : launchApp(entry.appId))
        );
        focusButton.appendChild(createIcon("task-icon", iconLabel, iconUrl, entry.appId));
        const name = document.createElement("span");
        name.className = "task-label";
        name.textContent = taskLabel;
        focusButton.appendChild(name);
        focusButton.title = taskLabel +
          (entry.pinned ? " (pinned)" : "") +
          " / right-click to " + (entry.pinned ? "unpin" : "pin");
        focusButton.addEventListener("contextmenu", (event) => {
          event.preventDefault();
          togglePin(entry.appId);
        });
        const group = document.createElement("span");
        group.className = "surface-actions";
        group.appendChild(focusButton);
        target.appendChild(group);
      });
    }

    async function togglePin(appId) {
      const pins = Array.isArray(state.pins) ? state.pins.slice() : [];
      const key = normalizeAppKey(appId);
      const index = pins.findIndex((candidate) => normalizeAppKey(candidate) === key);
      const status = document.getElementById("status-label");
      if (index >= 0) {
        pins.splice(index, 1);
        status.textContent = "unpinned";
      } else {
        pins.push(appId);
        status.textContent = "pinned";
      }
      status.className = "status ready";
      try {
        await putJSON("/api/taskbar/pins", {apps: pins});
        state.pins = pins;
        setFeedback(status.textContent, "ready");
        render();
      } catch (error) {
        status.textContent = "pin failed";
        status.className = "status warn";
      }
    }

    async function putJSON(path, body) {
      const response = await fetch(path, {
        method: "PUT",
        cache: "no-store",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(body)
      });
      if (!response.ok) {
        throw new Error(path + " returned " + response.status);
      }
      return response.json();
    }

    function workspaceZones() {
      const zones = Array.isArray(activeLayoutWorkspace().zones) ? activeLayoutWorkspace().zones : [];
      const zoneIds = zones
        .filter((zone) => text(zone.id, "") && zone.kind !== "chrome")
        .map((zone) => zone.id);
      return zoneIds.length ? zoneIds : ["primary", "secondary"];
    }

    function nextLayoutMode(mode) {
      const modes = ["freeform", "zones", "columns"];
      const current = modes.indexOf(mode);
      return modes[(current + 1) % modes.length];
    }

    function nextZone(zoneId) {
      const zones = workspaceZones();
      const index = zones.indexOf(zoneId);
      return zones[(index + 1) % zones.length];
    }

    function geometryLabel(geometry) {
      if (!geometry || geometry.width <= 0 || geometry.height <= 0) {
        return "";
      }
      return geometry.width + "x" + geometry.height + "@" + geometry.x + "," + geometry.y;
    }

    function surfaceAreaLabel(layout, zone) {
      const area = geometryLabel(layout.geometry);
      return area ? zone + " " + area : zone;
    }

    function layoutSettingsLabel() {
      const settings = state.layout.settings || {};
      const gaps = settings.gaps || {};
      const gapLabel = [gaps.outerHorizontal, gaps.outerVertical, gaps.innerHorizontal, gaps.innerVertical]
        .filter((value) => Number(value || 0) > 0)
        .length ? " gaps" : "";
      const master = settings.masterCount ? " n" + settings.masterCount : "";
      const ratio = settings.masterRatio ? " " + Math.round(settings.masterRatio * 100) + "%" : "";
      return text(settings.rule, "master_stack") + master + ratio + gapLabel;
    }

    function setFeedback(label, className) {
      state.feedback = {
        label,
        className: className || "ready",
        until: Date.now() + 3500
      };
    }

    function statusFromFeedback() {
      if (state.feedback.label && Date.now() < state.feedback.until) {
        return state.feedback;
      }
      return null;
    }

    function actionStatus(result) {
      return text(result && (result.status || result.decision), "accepted");
    }

    function setControlDisabled(id, disabled) {
      const element = document.getElementById(id);
      if (element) {
        element.disabled = disabled;
        element.setAttribute("aria-disabled", String(disabled));
      }
    }

    function renderWMControls() {
      const target = targetSurface();
      const hasTarget = Boolean(target && target.surfaceId);
      [
        "focus-prev-button",
        "focus-next-button",
        "promote-button",
        "move-left-button",
        "move-right-button",
        "move-up-button",
        "move-down-button",
        "swap-master-button",
        "move-zone-button",
        "float-button",
        "fullscreen-button",
        "maximize-button",
        "minimize-button",
        "close-focus-button"
      ].forEach((id) => setControlDisabled(id, !hasTarget));
      const targetLabel = document.getElementById("target-label");
      if (targetLabel) {
        const targetName = target ? text(target.title, text(target.appId, target.surfaceId)) : "no target";
        const targetZone = target ? text(target.zoneId, "primary") : "";
        targetLabel.textContent = target ? targetName : "no target";
        targetLabel.title = target ? targetName + " / " + targetZone + " / " + geometryLabel(target.geometry) : "no target";
      }
      const floatButton = document.getElementById("float-button");
      if (floatButton) {
        floatButton.textContent = target && target.floating ? "Tile" : "Float";
        floatButton.title = target ? "Toggle floating for " + text(target.title, text(target.appId, target.surfaceId)) : "Toggle floating";
      }
      const moveButton = document.getElementById("move-zone-button");
      if (moveButton && target) {
        moveButton.title = "Move " + text(target.title, target.surfaceId) + " to " + nextZone(text(target.zoneId, "primary"));
      }
      const fullscreenButton = document.getElementById("fullscreen-button");
      if (fullscreenButton) {
        fullscreenButton.textContent = target && target.fullscreen ? "Unfull" : "Full";
      }
      const maximizeButton = document.getElementById("maximize-button");
      if (maximizeButton) {
        maximizeButton.textContent = target && target.maximized ? "Unmax" : "Max";
      }
      const ruleLabel = document.getElementById("layout-rule-label");
      const settings = state.layout.settings || {};
      ruleLabel.textContent = layoutSettingsLabel();
      ruleLabel.title = "rule " + text(settings.rule, "master_stack") +
        " / mode " + text(state.layout.mode, "freeform") +
        " / revision " + Number(state.layout.revision || 0);
    }

    function render() {
      const launcher = launcherSurface();
      const showingApps = Boolean(launcher);
      document.querySelector(".panel").classList.toggle("apps-open", showingApps);
      const appsButton = document.getElementById("apps-button");
      appsButton.textContent = "Start";
      appsButton.title = showingApps ? "Close applications" : state.apps.length + " apps";
      appsButton.setAttribute("aria-pressed", showingApps ? "true" : "false");
      renderWorkspaces();
      const workSurfaces = state.surfaces.filter(isTaskbarWorkSurface);
      renderTaskbar(workSurfaces);
      const status = document.getElementById("status-label");
      const feedback = statusFromFeedback();
      if (feedback) {
        status.textContent = feedback.label;
        status.className = "status " + feedback.className;
      } else if (launcher) {
        status.textContent = "apps open";
        status.className = "status ready";
      } else {
        status.textContent = workSurfaces.length ? workSurfaces.length + " running" : "ready";
        status.className = "status " + (workSurfaces.length ? "ready" : "warn");
      }
      const layoutMode = text(state.layout.mode, "freeform");
      const layoutStatus = document.getElementById("layout-mode-button");
      layoutStatus.textContent = layoutMode + (state.layout.revision ? " r" + state.layout.revision : "");
      layoutStatus.title = layoutSettingsLabel() + " / zones: " + workspaceZones().join(" / ");
      renderWMControls();
    }

    function renderWorkspaces() {
      const target = document.getElementById("workspace-list");
      const activeId = activeWorkspaceId();
      const workspaces = allWorkspaces();
      const outputIds = new Set(workspaces.map(workspaceOutputId).filter(Boolean));
      const showOutputs = outputIds.size > 1;
      target.replaceChildren();
      workspaces.forEach((workspace) => {
        const workspaceId = text(workspace.id, "workspace-1");
        const outputId = workspaceOutputId(workspace);
        const button = document.createElement("button");
        button.type = "button";
        button.className = "workspace" + (workspaceId === activeId || workspace.active ? " active" : "");
        button.dataset.workspaceId = workspaceId;
        if (outputId) {
          button.dataset.outputId = outputId;
        }
        if (workspaceId === activeId) {
          button.id = "workspace-label";
        }
        const label = document.createElement("span");
        label.textContent = workspaceShortName(workspace);
        button.appendChild(label);
        if (showOutputs && outputId) {
          const output = document.createElement("span");
          output.className = "workspace-output";
          output.textContent = workspaceOutputShortName(outputId);
          button.appendChild(output);
        }
        const count = workspaceSurfaceCount(workspace);
        if (count) {
          const badge = document.createElement("span");
          badge.className = "workspace-count";
          badge.textContent = String(count);
          button.appendChild(badge);
        }
        button.title = workspaceId + (outputId ? " / " + outputId : "") + (count ? " / " + count + " surfaces" : "");
        target.appendChild(button);
      });
      const nextButton = document.createElement("button");
      nextButton.type = "button";
      nextButton.className = "workspace";
      nextButton.dataset.workspaceId = nextWorkspaceId();
      nextButton.textContent = "+";
      nextButton.title = nextWorkspaceId();
      target.appendChild(nextButton);
    }

    async function loadJSON(path) {
      const response = await fetch(path, {cache: "no-store"});
      if (!response.ok) {
        throw new Error(path + " returned " + response.status);
      }
      return response.json();
    }

    async function refresh() {
      try {
        const [catalog, surfaces, workspaces, layout, pins] = await Promise.all([
          loadJSON("/api/catalog/apps"),
          loadJSON("/api/surfaces"),
          loadJSON("/api/workspaces"),
          loadJSON("/api/layout"),
          loadJSON("/api/taskbar/pins")
        ]);
        state.apps = Array.isArray(catalog.apps) ? catalog.apps : [];
        state.surfaces = Array.isArray(surfaces.surfaces) ? surfaces.surfaces : [];
        state.pins = Array.isArray(pins.apps) ? pins.apps : [];
        state.layout = layout.layout || state.layout;
        if (Array.isArray(workspaces.workspaces) && workspaces.workspaces.length) {
          state.workspaceState = {
            currentWorkspaceId: text(workspaces.currentWorkspaceId, ""),
            currentOutputId: text(workspaces.currentOutputId, ""),
            workspaces: workspaces.workspaces
          };
          state.workspace = workspaces.workspaces.find((workspace) => workspace.id === state.workspaceState.currentWorkspaceId) ||
            workspaces.workspaces.find((workspace) => workspace.active) ||
            workspaces.workspaces[0];
        }
        render();
      } catch (error) {
        const status = document.getElementById("status-label");
        status.textContent = "offline";
        status.className = "status warn";
      }
    }

    async function postJSON(path, body) {
      const response = await fetch(path, {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(body)
      });
      if (!response.ok) {
        let payload = {};
        try {
          payload = await response.json();
        } catch (error) {
          payload = {};
        }
        const message = payload.error || path + " returned " + response.status;
        const error = new Error(message);
        error.errorClass = payload.errorClass || "";
        error.status = response.status;
        throw error;
      }
      return response.json();
    }

    async function launchApp(appId) {
      const status = document.getElementById("status-label");
      status.textContent = "launching";
      status.className = "status ready";
      try {
        const result = await postJSON("/api/catalog/launch", {appId});
        await refresh();
        setFeedback(actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = "launch failed";
        status.className = "status warn";
      }
    }

    async function activateTaskSurface(surface) {
      if (!surface || !surface.id) {
        return;
      }
      const status = document.getElementById("status-label");
      status.textContent = surface.minimized ? "restoring" : "focus";
      status.className = "status ready";
      try {
        if (surface.minimized) {
          await postJSON("/api/surfaces/action", {surfaceId: surface.id, action: "minimize", enabled: false});
        }
        const result = await postJSON("/api/surfaces/action", {surfaceId: surface.id, action: "focus"});
        await refresh();
        setFeedback((surface.minimized ? "restore " : "focus ") + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = surface.minimized ? "restore failed" : "focus failed";
        status.className = "status warn";
      }
    }

    async function actOnSurface(surfaceId, action, enabled) {
      const status = document.getElementById("status-label");
      status.textContent = action;
      status.className = "status ready";
      try {
        const body = {surfaceId, action};
        if (typeof enabled === "boolean") {
          body.enabled = enabled;
        }
        const result = await postJSON("/api/surfaces/action", body);
        await refresh();
        setFeedback(action + " " + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = error.errorClass === "backend_unsupported" ? action + " unsupported" : action + " failed";
        status.className = "status warn";
      }
    }

    async function assignZone(surfaceId, zoneId) {
      const status = document.getElementById("status-label");
      status.textContent = "zone";
      status.className = "status ready";
      try {
        const target = layoutSurface(surfaceId) || {};
        const result = await postJSON("/api/layout/action", {
          surfaceId,
          workspaceId: text(target.workspaceId, state.workspace.id),
          zoneId,
          action: "assignZone"
        });
        await refresh();
        setFeedback("zone " + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = "zone unsupported";
        status.className = "status warn";
      }
    }

    async function setLayoutMode(mode) {
      const status = document.getElementById("status-label");
      status.textContent = mode;
      status.className = "status ready";
      try {
        const result = await postJSON("/api/layout/action", {mode, action: "setMode"});
        await refresh();
        setFeedback("layout " + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = "layout unsupported";
        status.className = "status warn";
      }
    }

    async function activateWorkspace(workspaceId, outputId) {
      const status = document.getElementById("status-label");
      status.textContent = "workspace";
      status.className = "status ready";
      try {
        const targetWorkspaceId = text(workspaceId, nextWorkspaceId());
        if (state.layout.mode !== "zones") {
          await setLayoutMode("zones");
        }
        const targetOutputId = text(outputId, workspaceOutputId(workspaceById(targetWorkspaceId)));
        const body = {workspaceId: targetWorkspaceId, action: "activate"};
        if (targetOutputId) {
          body.outputId = targetOutputId;
        }
        const result = await postJSON("/api/workspaces/action", body);
        await refresh();
        setFeedback(text(result.currentWorkspaceId, targetWorkspaceId), "ready");
        render();
      } catch (error) {
        status.textContent = "workspace failed";
        status.className = "status warn";
      }
    }

    async function focusRelative(delta) {
      const surfaces = manageableLayoutSurfaces();
      const target = targetSurface();
      if (!surfaces.length || !target) {
        return;
      }
      const current = surfaces.findIndex((surface) => surface.surfaceId === target.surfaceId);
      const next = surfaces[(Math.max(current, 0) + delta + surfaces.length) % surfaces.length];
      if (next) {
        await promoteSurface(next.surfaceId);
      }
    }

    async function promoteSurface(surfaceId) {
      const status = document.getElementById("status-label");
      status.textContent = "promote";
      status.className = "status ready";
      try {
        const result = await postJSON("/api/layout/action", {surfaceId, action: "promote"});
        await refresh();
        setFeedback("promote " + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = "promote failed";
        status.className = "status warn";
      }
    }

    async function promoteTarget() {
      const target = targetSurface();
      if (!target) {
        return;
      }
      await promoteSurface(target.surfaceId);
    }

    async function moveTargetToNextZone() {
      const target = targetSurface();
      if (!target) {
        return;
      }
      await assignZone(target.surfaceId, nextZone(text(target.zoneId, "primary")));
    }

    async function moveTarget(direction) {
      const target = targetSurface();
      if (!target) {
        return;
      }
      const status = document.getElementById("status-label");
      status.textContent = "move " + direction;
      status.className = "status ready";
      try {
        const result = await postJSON("/api/layout/action", {surfaceId: target.surfaceId, action: "move", direction});
        await refresh();
        setFeedback("move " + direction + " " + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = "move failed";
        status.className = "status warn";
      }
    }

    async function swapMasterTarget() {
      const target = targetSurface();
      if (!target) {
        return;
      }
      const status = document.getElementById("status-label");
      status.textContent = "swap master";
      status.className = "status ready";
      try {
        const result = await postJSON("/api/layout/action", {surfaceId: target.surfaceId, action: "swapMaster"});
        await refresh();
        setFeedback("swap master " + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = "swap failed";
        status.className = "status warn";
      }
    }

    async function toggleTargetFloating() {
      const target = targetSurface();
      if (!target) {
        return;
      }
      const floating = !Boolean(target.floating);
      const status = document.getElementById("status-label");
      status.textContent = floating ? "floating" : "tiling";
      status.className = "status ready";
      try {
        const result = await postJSON("/api/layout/action", {surfaceId: target.surfaceId, floating, action: "setFloating"});
        await refresh();
        setFeedback((floating ? "float " : "tile ") + actionStatus(result), "ready");
        render();
      } catch (error) {
        status.textContent = "float failed";
        status.className = "status warn";
      }
    }

    async function actOnTarget(action, enabled) {
      if (unsupportedSurfaceActions.has(action)) {
        const status = document.getElementById("status-label");
        status.textContent = action + " unsupported";
        status.className = "status warn";
        return;
      }
      const target = targetSurface();
      if (target) {
        await actOnSurface(target.surfaceId, action, enabled);
      }
    }

    function toggleTargetState(action, field) {
      const target = targetSurface();
      if (!target) {
        return;
      }
      actOnSurface(target.surfaceId, action, !Boolean(target[field]));
    }

    async function resetLayout() {
      const mode = text(state.layout.mode, "freeform");
      await setLayoutMode(mode === "freeform" ? "zones" : mode);
    }

    async function toggleApps() {
      const status = document.getElementById("status-label");
      try {
        const surfaces = await loadJSON("/api/surfaces");
        state.surfaces = Array.isArray(surfaces.surfaces) ? surfaces.surfaces : [];
      } catch (_error) {
        // Use the most recent panel snapshot if the compositor read is transiently unavailable.
      }
      const launcher = launcherSurface();
      if (launcher) {
        status.textContent = "apps";
        status.className = "status ready";
        try {
          await postJSON("/api/surfaces/action", {surfaceId: launcher.id, action: "close"});
          await refresh();
        } catch (error) {
          status.textContent = "close failed";
          status.className = "status warn";
        }
        return;
      }
      await launchApp("shell-launcher");
    }

    function updateClock() {
      const now = new Date();
      document.getElementById("clock-label").textContent = now.toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit"
      });
    }

    document.getElementById("apps-button").addEventListener("click", toggleApps);
    document.getElementById("refresh-button").addEventListener("click", refresh);
    document.getElementById("operator-button").addEventListener("click", () => launchApp("shell-status"));
    document.getElementById("settings-button").addEventListener("click", () => launchApp("shell-settings"));
    document.getElementById("workspace-list").addEventListener("click", (event) => {
      const target = event.target.closest("button[data-workspace-id]");
      if (target) {
        activateWorkspace(target.dataset.workspaceId, target.dataset.outputId);
      }
    });
    document.getElementById("layout-mode-button").addEventListener("click", () => setLayoutMode(nextLayoutMode(text(state.layout.mode, "freeform"))));
    document.getElementById("focus-prev-button").addEventListener("click", () => focusRelative(-1));
    document.getElementById("focus-next-button").addEventListener("click", () => focusRelative(1));
    document.getElementById("promote-button").addEventListener("click", promoteTarget);
    document.getElementById("move-left-button").addEventListener("click", () => moveTarget("left"));
    document.getElementById("move-right-button").addEventListener("click", () => moveTarget("right"));
    document.getElementById("move-up-button").addEventListener("click", () => moveTarget("up"));
    document.getElementById("move-down-button").addEventListener("click", () => moveTarget("down"));
    document.getElementById("swap-master-button").addEventListener("click", swapMasterTarget);
    document.getElementById("move-zone-button").addEventListener("click", moveTargetToNextZone);
    document.getElementById("float-button").addEventListener("click", toggleTargetFloating);
    document.getElementById("fullscreen-button").addEventListener("click", () => toggleTargetState("fullscreen", "fullscreen"));
    document.getElementById("maximize-button").addEventListener("click", () => toggleTargetState("maximize", "maximized"));
    document.getElementById("minimize-button").addEventListener("click", () => actOnTarget("minimize"));
    document.getElementById("close-focus-button").addEventListener("click", () => actOnTarget("close"));
    document.getElementById("reset-layout-button").addEventListener("click", resetLayout);
    updateClock();
    refresh();
    setInterval(updateClock, 30000);
    setInterval(refresh, 3000);
  </script>
</body>
</html>`;

export interface BackgroundHTMLOptions {
  readonly includeTaskbar: boolean;
}

export function backgroundHTML(options: BackgroundHTMLOptions): string {
  const bodyClass = options.includeTaskbar ? 'background with-taskbar' : 'background';
  const rows = options.includeTaskbar ? '1fr var(--agora-panel-height)' : '1fr';
  const taskbarHTML = options.includeTaskbar ? `
  <nav class="taskbar" aria-label="Agora DE fallback taskbar">
    <span class="badge">agora-de</span>
    <span class="slot">shell: dock</span>
    <span class="slot">workspace 1</span>
    <span class="slot">ready</span>
  </nav>` : '';
  return `<!doctype html>
<html>
<head>
  <title>agora-de shell</title>
  <style>
__AGORA_THEME_CSS__
${componentCSS}
    html,
    body {
      background: var(--agora-bg);
      color: var(--agora-fg);
      font: var(--agora-font-background);
      height: 100%;
      margin: 0;
    }
    body {
      box-sizing: border-box;
      display: grid;
      grid-template-rows: ${rows};
      min-height: 100vh;
    }
    .stage {
      align-items: center;
      display: flex;
      gap: 18px;
      padding: 0 28px;
    }
    .mark {
      background: var(--agora-evidence-accent);
      border-radius: var(--agora-radius-control);
      height: 40px;
      width: 40px;
    }
    .taskbar {
      align-items: center;
      background: var(--agora-surface);
      border-top: 4px solid var(--agora-evidence-accent);
      box-shadow: inset 0 1px 0 var(--agora-border-subtle);
      box-sizing: border-box;
      display: flex;
      gap: 18px;
      min-height: var(--agora-panel-height);
      padding: 0 28px;
    }
    .badge {
      align-items: center;
      background: var(--agora-evidence-strong);
      border-radius: var(--agora-radius-control);
      color: var(--agora-bg);
      display: inline-flex;
      height: var(--agora-control-height);
      justify-content: center;
      min-width: 132px;
      padding: 0 16px;
    }
    .slot {
      align-items: center;
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      display: inline-flex;
      height: 40px;
      padding: 0 14px;
    }
  </style>
</head>
<body class="${bodyClass}" data-surface="__AGORA_SURFACE__">
  <main class="stage">
    <span class="mark"></span>
    <span>agora-de shell: __AGORA_SURFACE__</span>
  </main>${taskbarHTML}
  <script>${liveThemeScript}</script>
</body>
</html>`;
}
