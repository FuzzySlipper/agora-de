import type { SettingsPageDefinition, SettingsPageRegistration } from '@agora-de/domain';
import type { DisplayHeadConfiguration, DisplaySettingsState, DisplayTopology } from '@agora-de/protocol';

export interface DisplayModeChoice {
  readonly id: string;
  readonly label: string;
  readonly preferred: boolean;
}

export function displayModeChoices(head: DisplayHeadConfiguration): readonly DisplayModeChoice[] {
  return head.modes.map((mode) => ({
    id: mode.id,
    label: `${mode.width} × ${mode.height} — ${(mode.refreshMillihz / 1000).toFixed(2)} Hz${mode.preferred ? ' (preferred)' : ''}`,
    preferred: mode.preferred,
  }));
}

export function updateDisplayHead(
  topology: DisplayTopology,
  headId: string,
  update: Partial<Pick<DisplayHeadConfiguration, 'enabled' | 'currentModeId' | 'x' | 'y' | 'scaleMilli' | 'transform' | 'adaptiveSync'>>,
): DisplayTopology {
  if (!topology.heads.some((head) => head.id === headId)) throw new Error(`unknown display head: ${headId}`);
  return {...topology, heads: topology.heads.map((head) => head.id === headId ? {...head, ...update} : head)};
}

export function displaySummary(state: Pick<DisplaySettingsState, 'active'>): string {
  const connected = state.active.heads.filter((head) => head.connected);
  const enabled = connected.filter((head) => head.enabled);
  return `${enabled.length} of ${connected.length} connected displays enabled`;
}

export const displaysSettingsPage: SettingsPageDefinition = {
  moduleId: 'displays',
  uiEntryPoint: 'settings-displays',
  title: 'Displays',
  renderPanel: () => `
    <style>
      .display-status { background: var(--agora-surface); border: 1px solid var(--agora-border); border-radius: var(--agora-radius-control); color: var(--agora-text-muted); padding: 12px 14px; }
      .display-confirm { background: color-mix(in srgb, var(--agora-warning) 13%, var(--agora-surface)); border: 2px solid var(--agora-warning); border-radius: var(--agora-radius-control); display: grid; gap: 10px; padding: 16px; }
      .display-confirm[hidden] { display: none; }
      .display-confirm-actions { display: flex; flex-wrap: wrap; gap: 9px; }
      .display-arrangement { background: var(--agora-surface); border: 1px solid var(--agora-border); border-radius: var(--agora-radius-control); min-height: 230px; overflow: hidden; position: relative; }
      .display-tile { background: var(--agora-surface-raised); border: 2px solid var(--agora-border); border-radius: 8px; color: var(--agora-fg); cursor: grab; min-height: 54px; min-width: 90px; overflow: hidden; padding: 8px; position: absolute; text-align: left; touch-action: none; }
      .display-tile[aria-pressed="true"] { border-color: var(--agora-accent); box-shadow: 0 0 0 2px color-mix(in srgb, var(--agora-accent) 24%, transparent); }
      .display-tile:disabled { cursor: default; opacity: .55; }
      .display-grid { display: grid; gap: 14px; }
      .display-card { background: var(--agora-surface); border: 1px solid var(--agora-border); border-radius: var(--agora-radius-control); display: grid; gap: 14px; padding: 16px; }
      .display-card[aria-current="true"] { border-color: var(--agora-accent); }
      .display-card-header { align-items: start; display: flex; gap: 12px; justify-content: space-between; }
      .display-card-title { display: grid; gap: 4px; }
      .display-fields { display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); }
      .display-field { color: var(--agora-text-muted); display: grid; font-size: 12px; gap: 6px; }
      .display-field input, .display-field select { background: var(--agora-bg); border: 2px solid var(--agora-border); border-radius: var(--agora-radius-control); color: var(--agora-fg); min-height: 40px; padding: 7px 9px; width: 100%; }
      .display-check { align-items: center; color: var(--agora-fg); display: flex; gap: 8px; }
      .display-check input { height: 20px; width: 20px; }
      @media (max-width: 720px) { .display-arrangement { min-height: 180px; } }
    </style>
    <section class="settings-module-panel" data-settings-module="displays">
      <p class="display-status" id="display-status" role="status">Loading display topology…</p>
      <p class="setting-detail">Primary display selection is not exposed by the supported output-management protocol and is not configurable in this release.</p>
      <section class="display-confirm" id="display-confirm" role="alertdialog" aria-modal="true" aria-labelledby="display-confirm-title" aria-describedby="display-confirm-countdown" hidden>
        <strong id="display-confirm-title">Keep these display settings?</strong>
        <span id="display-confirm-countdown" role="status" aria-live="assertive">The previous configuration will be restored automatically.</span>
        <div class="display-confirm-actions"><button class="action primary" id="display-keep" type="button">Keep</button><button class="action" id="display-revert" type="button">Revert now</button></div>
      </section>
      <div class="display-arrangement" id="display-arrangement" role="group" aria-label="Display arrangement. Select a display, drag it, or use arrow keys to move it."></div>
      <div class="display-grid" id="display-controls"></div>
    </section>`,
  renderClientController: () => String.raw`(api) => {
      const arrangement = api.content.querySelector("#display-arrangement");
      const controls = api.content.querySelector("#display-controls");
      const status = api.content.querySelector("#display-status");
      const confirmation = api.content.querySelector("#display-confirm");
      const countdown = api.content.querySelector("#display-confirm-countdown");
      const keep = api.content.querySelector("#display-keep");
      const revert = api.content.querySelector("#display-revert");
      let selected = "";
      let timer = 0;
      let topologyTimer = 0;
      let polling = false;
      let confirmationBusy = false;
      let confirmationWasOpen = false;
      let confirmationReturnFocus = null;

      function element(tag, className, text) {
        const node = document.createElement(tag);
        if (className) node.className = className;
        if (text !== undefined) node.textContent = text;
        return node;
      }
      function draft() { return api.getDraft(); }
      function state() { return api.getModuleState(); }
      function head(id) { return draft() && draft().heads.find((candidate) => candidate.id === id); }
      function edit(id, field, value) {
        const target = head(id); if (!target) return;
        target[field] = value; selected = id; api.markDirty();
      }
      function logicalSize(output) {
        const mode = output.modes.find((candidate) => candidate.id === output.currentModeId) || output.modes[0];
        if (!mode) return {width: 800, height: 600};
        const rotated = ["rotate_90", "rotate_270", "flipped_90", "flipped_270"].includes(output.transform);
        return {width: (rotated ? mode.height : mode.width) * 1000 / output.scaleMilli, height: (rotated ? mode.width : mode.height) * 1000 / output.scaleMilli};
      }
      function renderArrangement(outputs) {
        arrangement.replaceChildren();
        const active = outputs.filter((output) => output.connected && output.enabled);
        if (!active.length) return;
        const rects = active.map((output) => ({output, ...logicalSize(output)}));
        const minX = Math.min(...rects.map((rect) => rect.output.x));
        const minY = Math.min(...rects.map((rect) => rect.output.y));
        const maxX = Math.max(...rects.map((rect) => rect.output.x + rect.width));
        const maxY = Math.max(...rects.map((rect) => rect.output.y + rect.height));
        const spanX = Math.max(1, maxX - minX); const spanY = Math.max(1, maxY - minY);
        rects.forEach((rect) => {
          const tile = element("button", "display-tile", rect.output.identity.name || rect.output.id);
          tile.type = "button"; tile.dataset.headId = rect.output.id;
          tile.setAttribute("aria-pressed", String(selected === rect.output.id));
          tile.setAttribute("aria-label", (rect.output.identity.description || rect.output.id) + ". Position " + rect.output.x + ", " + rect.output.y + ". Use arrow keys to move.");
          tile.style.left = (4 + ((rect.output.x - minX) / spanX) * 76) + "%";
          tile.style.top = (6 + ((rect.output.y - minY) / spanY) * 66) + "%";
          tile.style.width = Math.max(90, (rect.width / spanX) * arrangement.clientWidth * .76) + "px";
          tile.style.height = Math.max(54, (rect.height / spanY) * arrangement.clientHeight * .66) + "px";
          tile.addEventListener("click", () => { selected = rect.output.id; render(); api.content.querySelector('[data-display-card="' + CSS.escape(selected) + '"]')?.scrollIntoView({block: "nearest"}); });
          tile.addEventListener("keydown", (event) => {
            const movement = {ArrowLeft: [-10, 0], ArrowRight: [10, 0], ArrowUp: [0, -10], ArrowDown: [0, 10]}[event.key];
            if (!movement) return; event.preventDefault();
            edit(rect.output.id, "x", rect.output.x + movement[0]); edit(rect.output.id, "y", rect.output.y + movement[1]);
          });
          tile.addEventListener("pointerdown", (event) => {
            const startX = event.clientX; const startY = event.clientY; const originX = rect.output.x; const originY = rect.output.y;
            tile.setPointerCapture(event.pointerId);
            const move = (next) => { edit(rect.output.id, "x", Math.round(originX + (next.clientX - startX) * spanX / Math.max(1, arrangement.clientWidth * .76))); edit(rect.output.id, "y", Math.round(originY + (next.clientY - startY) * spanY / Math.max(1, arrangement.clientHeight * .66))); };
            tile.addEventListener("pointermove", move);
            tile.addEventListener("pointerup", () => tile.removeEventListener("pointermove", move), {once: true});
          });
          arrangement.appendChild(tile);
        });
      }
      function field(label, control) { const wrapper = element("label", "display-field"); wrapper.append(element("span", "", label), control); return wrapper; }
      function selectField(output, label, property, choices) {
        const select = element("select"); select.disabled = !output.enabled;
        choices.forEach((choice) => { const option = element("option", "", choice.label); option.value = choice.value; option.selected = String(output[property]) === choice.value; select.appendChild(option); });
        select.addEventListener("change", () => edit(output.id, property, property === "scaleMilli" ? Number(select.value) : select.value));
        return field(label, select);
      }
      function numberField(output, label, property) {
        const input = element("input"); input.type = "number"; input.value = String(output[property]); input.disabled = !output.enabled; input.step = "10";
        input.addEventListener("change", () => edit(output.id, property, Number(input.value))); return field(label, input);
      }
      function renderControls(outputs) {
        controls.replaceChildren();
        outputs.forEach((output) => {
          const card = element("section", "display-card"); card.dataset.displayCard = output.id; card.setAttribute("aria-current", String(selected === output.id));
          const header = element("div", "display-card-header"); const title = element("span", "display-card-title");
          title.append(element("strong", "", output.identity.description || output.id), element("span", "setting-detail", [output.identity.make, output.identity.model].filter(Boolean).join(" ") || output.id));
          const enabledLabel = element("label", "display-check"); const enabled = element("input"); enabled.type = "checkbox"; enabled.checked = output.enabled; enabled.disabled = !output.connected; enabled.addEventListener("change", () => edit(output.id, "enabled", enabled.checked)); enabledLabel.append(enabled, document.createTextNode("Enabled"));
          header.append(title, enabledLabel); card.appendChild(header);
          const fields = element("div", "display-fields");
          fields.append(
            selectField(output, "Resolution and refresh", "currentModeId", output.modes.map((mode) => ({value: mode.id, label: mode.width + " × " + mode.height + " — " + (mode.refreshMillihz / 1000).toFixed(2) + " Hz" + (mode.preferred ? " (preferred)" : "")}))),
            selectField(output, "Scale", "scaleMilli", Array.from(new Set([output.scaleMilli, 1000, 1250, 1500, 1750, 2000])).sort((a,b) => a-b).map((scale) => ({value: String(scale), label: (scale / 10) + "%"}))),
            selectField(output, "Rotation", "transform", ["normal", "rotate_90", "rotate_180", "rotate_270", "flipped", "flipped_90", "flipped_180", "flipped_270"].map((value) => ({value, label: value.replaceAll("_", " ")}))),
            numberField(output, "Horizontal position", "x"), numberField(output, "Vertical position", "y")
          );
          if (state()?.capabilities?.adaptiveSync) { const adaptiveLabel = element("label", "display-check"); const adaptive = element("input"); adaptive.type = "checkbox"; adaptive.checked = output.adaptiveSync; adaptive.disabled = !output.enabled; adaptive.addEventListener("change", () => edit(output.id, "adaptiveSync", adaptive.checked)); adaptiveLabel.append(adaptive, document.createTextNode("Adaptive sync")); fields.appendChild(adaptiveLabel); }
          card.appendChild(fields); card.addEventListener("click", () => { selected = output.id; }); controls.appendChild(card);
        });
      }
      function renderLease(moduleState) {
        const lease = moduleState && moduleState.lease;
        const pending = lease && lease.state === "pending";
        confirmation.hidden = !pending;
        if (timer) { clearInterval(timer); timer = 0; }
        if (!pending) { if (confirmationWasOpen && confirmationReturnFocus && confirmationReturnFocus.isConnected) confirmationReturnFocus.focus(); confirmationWasOpen = false; confirmationReturnFocus = null; return; }
        if (!confirmationWasOpen) { confirmationReturnFocus = document.activeElement; confirmationWasOpen = true; queueMicrotask(() => keep.focus()); }
        const update = () => { const remaining = Math.max(0, lease.deadlineUnixMillis - Date.now()); countdown.textContent = "Reverting in " + Math.ceil(remaining / 1000) + " seconds unless you keep these settings."; if (!remaining) { clearInterval(timer); timer = 0; setTimeout(api.reload, 250); } };
        update(); timer = setInterval(update, 250);
        keep.disabled = confirmationBusy; revert.disabled = confirmationBusy;
      }
      function render() {
        const moduleState = state(); const topology = draft();
        if (!moduleState || !topology) return;
        if (!selected || !topology.heads.some((output) => output.id === selected)) selected = topology.heads.find((output) => output.enabled)?.id || topology.heads[0]?.id || "";
        const enabled = topology.heads.filter((output) => output.connected && output.enabled).length;
        const connected = topology.heads.filter((output) => output.connected).length;
        status.textContent = enabled + " of " + connected + " connected displays enabled. " + moduleState.reconciliation.detail;
        renderArrangement(topology.heads); renderControls(topology.heads); renderLease(moduleState);
      }
      async function confirmationAction(operation) {
        const moduleState = state(); const lease = moduleState && moduleState.lease; if (!lease || confirmationBusy) return;
        confirmationBusy = true; renderLease(moduleState);
        try { const result = await api.post(operation, {contractVersion: moduleState.contractVersion, transactionId: lease.transactionId}); api.acceptResult(result, operation === "keep" ? "Display configuration kept" : "Previous display configuration restored"); }
        catch (error) { api.setError(error.settings || {code: "apply_failed", message: error.message}); }
        finally { confirmationBusy = false; render(); }
      }
      async function refreshTopology() {
        if (polling || confirmationBusy) return;
        polling = true;
        try {
          const latest = await api.load(); const previous = state();
          if (!previous || latest.revision === previous.revision) return;
          const edited = draft(); const dirty = api.isDirty();
          const projected = !dirty ? JSON.parse(JSON.stringify(latest.active)) : {
            ...latest.active,
            heads: latest.active.heads.map((active) => {
              const prior = edited.heads.find((candidate) => candidate.id === active.id); if (!prior) return active;
              const mode = active.modes.some((candidate) => candidate.id === prior.currentModeId) ? prior.currentModeId : active.currentModeId;
              return {...active, enabled: prior.enabled, currentModeId: mode, x: prior.x, y: prior.y, scaleMilli: prior.scaleMilli, transform: prior.transform, adaptiveSync: prior.adaptiveSync};
            }),
          };
          api.acceptExternalState(latest, projected, dirty, dirty ? "Display topology changed; review the updated draft" : "Display topology updated");
        } catch (_error) {} finally { polling = false; }
      }
      keep.addEventListener("click", () => confirmationAction("keep")); revert.addEventListener("click", () => confirmationAction("revert"));
      confirmation.addEventListener("keydown", (event) => { if (event.key !== "Tab") return; const first=keep,last=revert; if(event.shiftKey && document.activeElement===first){event.preventDefault();last.focus();}else if(!event.shiftKey && document.activeElement===last){event.preventDefault();first.focus();} });
      topologyTimer = setInterval(refreshTopology, 2000);
      return { prepareApply(request) { request.confirmationTimeoutMillis = 15000; }, render, destroy() { if (timer) clearInterval(timer); if (topologyTimer) clearInterval(topologyTimer); } };
    }`,
};

export const displaysSettingsPageRegistration: SettingsPageRegistration = {
  uiEntryPoint: displaysSettingsPage.uiEntryPoint,
  load: async () => displaysSettingsPage,
};

export function assertDisplaySettingsFixtures(): void {
  const head: DisplayHeadConfiguration = {
    id: 'DP-1', identity: {name: 'DP-1', description: 'Fixture', make: 'Agora', model: 'Panel', serialNumber: '1', physicalWidthMm: 600, physicalHeightMm: 340},
    connected: true, enabled: true,
    modes: [{id: '1440p-60', width: 2560, height: 1440, refreshMillihz: 59951, preferred: true}, {id: '1440p-120', width: 2560, height: 1440, refreshMillihz: 120000, preferred: false}],
    currentModeId: '1440p-60', x: 0, y: 0, scaleMilli: 1250, transform: 'rotate_90', adaptiveSync: false,
  };
  const topology = {serial: 9, heads: [head, {...head, id: 'DP-2', connected: false, enabled: false}]};
  if (displayModeChoices(head).length !== 2 || !displayModeChoices(head)[0]?.label.includes('59.95 Hz')) throw new Error('display modes must preserve refresh choices');
  const updated = updateDisplayHead(topology, 'DP-1', {scaleMilli: 1500, transform: 'normal'});
  if (updated.heads[0]?.scaleMilli !== 1500 || topology.heads[0]?.scaleMilli !== 1250) throw new Error('display edits must be immutable and preserve fractional scale fixtures');
  const state = {active: topology};
  if (displaySummary(state) !== '1 of 1 connected displays enabled') throw new Error('display summary must exclude disconnected heads');
  const arranged = updateDisplayHead(topology, 'DP-1', {x: 1920, y: 10, currentModeId: '1440p-120', enabled: false});
  if (arranged.heads[0]?.x !== 1920 || arranged.heads[0]?.currentModeId !== '1440p-120' || arranged.heads[0]?.enabled) throw new Error('arrangement, mode, refresh, and enabled edits must project together');
  let unknownRejected = false;
  try { updateDisplayHead(topology, 'missing', {x: 1}); } catch { unknownRejected = true; }
  if (!unknownRejected) throw new Error('hotplug removal must reject edits for a missing head');
  const controller = displaysSettingsPage.renderClientController?.() ?? '';
  for (const marker of ['pointerdown', 'keydown', 'confirmationTimeoutMillis', '"keep"', '"revert"', 'refreshTopology']) {
    if (!controller.includes(marker)) throw new Error(`display controller fixture is missing ${marker}`);
  }
}
