import type { SettingsPageDefinition, SettingsPageRegistration } from '@agora-de/domain';
import type { WindowManagementSettings, WindowWorkspaceSummary } from '@agora-de/protocol';

export function windowManagementSummary(workspaces: readonly WindowWorkspaceSummary[]): string {
  const active = workspaces.find((workspace) => workspace.active);
  if (!active) return `${workspaces.length} workspaces`;
  return `${active.name} active on ${active.outputId || 'the current output'} with ${active.surfaceCount} windows`;
}

export function editWindowManagementSetting(
  settings: WindowManagementSettings,
  update: Partial<WindowManagementSettings>,
): WindowManagementSettings {
  return {...settings, ...update, gaps: update.gaps ? {...settings.gaps, ...update.gaps} : settings.gaps};
}

export const windowManagementSettingsPage: SettingsPageDefinition = {
  moduleId: 'window-management',
  uiEntryPoint: 'settings-window-management',
  title: 'Window Management',
  renderPanel: () => `
    <style>
      .wm-settings-grid { display:grid; gap:14px; grid-template-columns:repeat(auto-fit,minmax(220px,1fr)); }
      .wm-settings-card { background:var(--agora-surface); border:1px solid var(--agora-border); border-radius:var(--agora-radius-control); display:grid; gap:12px; padding:16px; }
      .wm-settings-card h3 { font-size:16px; }
      .wm-settings-field { color:var(--agora-text-muted); display:grid; font-size:12px; gap:6px; }
      .wm-settings-field input,.wm-settings-field select { background:var(--agora-bg); border:2px solid var(--agora-border); border-radius:var(--agora-radius-control); color:var(--agora-fg); min-height:42px; padding:7px 9px; width:100%; }
      .wm-settings-check { align-items:center; color:var(--agora-fg); display:flex; gap:9px; min-height:42px; }
      .wm-settings-check input { height:20px; width:20px; }
      .wm-workspaces { display:grid; gap:8px; list-style:none; margin:0; padding:0; }
      .wm-workspace { border:1px solid var(--agora-border-subtle); border-radius:var(--agora-radius-control); display:flex; justify-content:space-between; padding:10px 12px; }
    </style>
    <section class="settings-module-panel" data-settings-module="window-management">
      <p class="display-status" id="wm-settings-status" role="status">Loading layout authority…</p>
      <div class="wm-settings-grid">
        <section class="wm-settings-card"><h3>Layout</h3>
          <label class="wm-settings-field">Mode<select data-wm-field="mode"><option value="freeform">Freeform</option><option value="zones">Zones</option><option value="columns">Columns</option></select></label>
          <label class="wm-settings-field">Rule<select data-wm-field="rule"><option value="master_stack">Master and stack</option><option value="dwindle">Dwindle</option><option value="zones">Zones</option></select></label>
          <label class="wm-settings-field">Master windows<input data-wm-field="masterCount" type="number" min="1" max="8" step="1"></label>
          <label class="wm-settings-field">Master ratio (%)<input data-wm-field="masterRatio" type="number" min="10" max="90" step="5"></label>
          <label class="wm-settings-check"><input data-wm-field="smartGaps" type="checkbox">Hide outer gaps for one tiled window</label>
        </section>
        <section class="wm-settings-card"><h3>Gaps</h3>
          <label class="wm-settings-field">Outer horizontal<input data-wm-field="gaps.outerHorizontal" type="number" min="0" max="128" step="1"></label>
          <label class="wm-settings-field">Outer vertical<input data-wm-field="gaps.outerVertical" type="number" min="0" max="128" step="1"></label>
          <label class="wm-settings-field">Inner horizontal<input data-wm-field="gaps.innerHorizontal" type="number" min="0" max="128" step="1"></label>
          <label class="wm-settings-field">Inner vertical<input data-wm-field="gaps.innerVertical" type="number" min="0" max="128" step="1"></label>
        </section>
        <section class="wm-settings-card"><h3>Workspaces</h3><p class="setting-detail">Workspace activation stays in the shell; this page shows authoritative layout membership.</p><ul class="wm-workspaces" id="wm-workspaces"></ul></section>
      </div>
    </section>`,
  renderClientController: () => String.raw`(api) => {
      const status = api.content.querySelector("#wm-settings-status");
      const workspaceList = api.content.querySelector("#wm-workspaces");
      const controls = Array.from(api.content.querySelectorAll("[data-wm-field]"));
      let polling = false; let timer = 0;
      function at(source,path) { return path.split(".").reduce((value,key) => value == null ? undefined : value[key],source); }
      function set(source,path,value) { const keys=path.split("."); let cursor=source; keys.slice(0,-1).forEach((key)=>{cursor=cursor[key];}); cursor[keys[keys.length-1]]=value; }
      controls.forEach((control) => control.addEventListener("change", () => {
        const path=control.dataset.wmField; let value=control.type === "checkbox" ? control.checked : control.value;
        if (control.type === "number") value=Number(value); if (path === "masterRatio") value=value/100;
        set(api.getDraft(),path,value); api.markDirty();
      }));
      function render() {
        const moduleState=api.getModuleState(); const draft=api.getDraft(); if (!moduleState || !draft) return;
        controls.forEach((control)=>{let value=at(draft,control.dataset.wmField); if(control.dataset.wmField==="masterRatio") value=Math.round(value*100); if(control.type==="checkbox") control.checked=Boolean(value); else control.value=String(value);});
        const active=moduleState.workspaces.find((workspace)=>workspace.active); status.textContent=active ? active.name+" active on "+(active.outputId||"current output")+" with "+active.surfaceCount+" windows" : moduleState.workspaces.length+" workspaces";
        workspaceList.replaceChildren(); moduleState.workspaces.forEach((workspace)=>{const item=document.createElement("li");item.className="wm-workspace";const name=document.createElement("strong");name.textContent=workspace.name+(workspace.active?" (active)":"");const detail=document.createElement("span");detail.className="setting-detail";detail.textContent=workspace.surfaceCount+" windows"+(workspace.outputId?" · "+workspace.outputId:"");item.append(name,detail);workspaceList.appendChild(item);});
      }
      async function poll() { if(polling)return; polling=true; try{const latest=await api.load();const current=api.getModuleState();if(current&&latest.revision!==current.revision){api.acceptExternalState(latest,api.isDirty()?api.getDraft():JSON.parse(JSON.stringify(latest.active)),api.isDirty(),api.isDirty()?"Layout changed externally; review your draft":"Layout updated");}}catch(_error){}finally{polling=false;} }
      timer=setInterval(poll,2000); return {render,destroy(){if(timer)clearInterval(timer);}};
    }`,
};

export const windowManagementSettingsPageRegistration: SettingsPageRegistration = {
  uiEntryPoint: windowManagementSettingsPage.uiEntryPoint,
  load: async () => windowManagementSettingsPage,
};

export function assertWindowManagementFixtures(): void {
  const settings: WindowManagementSettings = {mode:'columns',rule:'master_stack',gaps:{outerHorizontal:4,outerVertical:4,innerHorizontal:8,innerVertical:8},masterCount:1,masterRatio:.55,smartGaps:true};
  const edited=editWindowManagementSetting(settings,{masterCount:2,masterRatio:.6,gaps:{...settings.gaps,innerHorizontal:12}});
  if(edited.masterCount!==2||edited.masterRatio!==.6||edited.gaps.innerHorizontal!==12||settings.masterCount!==1)throw new Error('window-management edits must preserve immutable draft semantics');
  if(!windowManagementSummary([{id:'workspace-1',name:'Workspace 1',outputId:'HDMI-A-1',active:true,surfaceCount:2}]).includes('2 windows'))throw new Error('workspace summary must expose authoritative membership');
  const source=windowManagementSettingsPage.renderClientController?.()??'';for(const marker of ['dataset.wmField','setInterval','acceptExternalState'])if(!source.includes(marker))throw new Error(`window-management controller missing ${marker}`);
}
