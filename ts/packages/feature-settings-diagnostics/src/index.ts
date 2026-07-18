import type { SettingsPageDefinition, SettingsPageRegistration } from '@agora-de/domain';
import type { DiagnosticsSettingsState } from '@agora-de/protocol';

export interface DiagnosticsSettingsViewModel {
  readonly overlayEnabled: boolean;
  readonly overlayActive: boolean;
  readonly serviceState: string;
  readonly available: boolean;
}

export function diagnosticsSettingsViewModel(state: DiagnosticsSettingsState): DiagnosticsSettingsViewModel {
  return {
    overlayEnabled: state.active.diagnosticOverlayEnabled,
    overlayActive: state.service.active,
    serviceState: `${state.service.enabledState} / ${state.service.activeState}`,
    available: state.availability.state === 'available',
  };
}

export const diagnosticsSettingsPage: SettingsPageDefinition = {
  moduleId: 'diagnostics',
  uiEntryPoint: 'settings-diagnostics',
  title: 'Diagnostics & About',
  renderPanel: () => `
    <style>.diagnostics-about{display:grid;gap:14px}.diagnostics-health{display:grid;gap:8px}.diagnostics-component{background:var(--agora-surface);border:1px solid var(--agora-border-subtle);display:grid;gap:4px;grid-template-columns:minmax(160px,1fr) auto;padding:11px}.diagnostics-component .setting-detail{grid-column:1/-1}.diagnostics-component[data-state="unavailable"]{border-color:var(--agora-warning)}</style>
    <section class="settings-module-panel" data-settings-module="diagnostics">
      <div class="setting-row">
        <span class="setting-copy">
          <strong class="setting-title">Debug overlay</strong>
          <span class="setting-detail"><span data-settings-text="service.enabledState">loading</span> / <span data-settings-text="service.activeState">loading</span></span>
        </span>
        <button class="toggle" id="overlay-toggle" data-settings-field="diagnosticOverlayEnabled" type="button" aria-label="Debug overlay" aria-pressed="false"></button>
      </div>
      <section class="diagnostics-about"><h3>About</h3><p><strong id="diagnostics-version">Agora DE</strong> · Settings schema <span id="diagnostics-schema">1</span></p><h3>Service health</h3><div class="diagnostics-health" id="diagnostics-health"></div><p id="diagnostics-export-status" role="status" aria-live="polite"></p><button type="button" id="diagnostics-export">Export bounded support bundle</button><p class="setting-detail">Includes only product versions and allowlisted service states. It excludes environment values, window titles, clipboard data, user files, and journal logs.</p></section>
    </section>`,
  renderClientController: () => String.raw`(api)=>{const health=api.content.querySelector("#diagnostics-health"),version=api.content.querySelector("#diagnostics-version"),schema=api.content.querySelector("#diagnostics-schema"),button=api.content.querySelector("#diagnostics-export"),status=api.content.querySelector("#diagnostics-export-status");function render(){const state=api.getModuleState();if(!state)return;version.textContent="Agora DE "+state.productVersion;schema.textContent=String(state.settingsSchemaVersion);health.replaceChildren();state.components.forEach((component)=>{const row=document.createElement("div");row.className="diagnostics-component";row.dataset.state=component.state;const label=document.createElement("strong");label.textContent=component.label;const value=document.createElement("span");value.textContent=component.state+" · "+component.version;const detail=document.createElement("span");detail.className="setting-detail";detail.textContent=component.detail+(component.state!=="available"?". "+component.recovery:"");row.append(label,value,detail);health.appendChild(row);});}button.addEventListener("click",()=>{try{const state=api.getModuleState(),json=JSON.stringify(state.supportBundle,null,2);if(json.length>16384)throw new Error("bundle exceeded the 16 KiB limit");const url=URL.createObjectURL(new Blob([json],{type:"application/json"})),anchor=document.createElement("a");anchor.href=url;anchor.download="agora-de-support-bundle.json";anchor.click();URL.revokeObjectURL(url);status.textContent="Support bundle exported.";}catch(error){status.textContent="Support bundle export failed: "+error.message;}});return {render};}`,
};

export const diagnosticsSettingsPageRegistration: SettingsPageRegistration = {
  uiEntryPoint: diagnosticsSettingsPage.uiEntryPoint,
  load: async () => diagnosticsSettingsPage,
};

export function assertDiagnosticsFixtures(): void {
  const source=diagnosticsSettingsPage.renderClientController?.()??'';
  for(const marker of ['supportBundle','16384','URL.createObjectURL','component.recovery'])if(!source.includes(marker))throw new Error(`diagnostics controller missing ${marker}`);
}
