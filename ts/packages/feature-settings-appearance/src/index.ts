import type { SettingsPageDefinition, SettingsPageRegistration } from '@agora-de/domain';
import type { AppearanceThemeSummary } from '@agora-de/protocol';

export function themeSwatch(theme: AppearanceThemeSummary): {background:string;surface:string;accent:string} {
  return {background:theme.tokens['--agora-bg']??'#000',surface:theme.tokens['--agora-surface']??'#222',accent:theme.tokens['--agora-accent']??'#fff'};
}

export const appearanceSettingsPage: SettingsPageDefinition = {
  moduleId:'appearance',uiEntryPoint:'settings-appearance',title:'Appearance',
  renderPanel:()=>`<style>
    .appearance-themes{display:grid;gap:12px;grid-template-columns:repeat(auto-fit,minmax(210px,1fr))}.appearance-theme{background:var(--agora-surface);border:2px solid var(--agora-border);border-radius:var(--agora-radius-control);display:grid;gap:10px;padding:14px}.appearance-theme:has(input:checked){border-color:var(--agora-accent)}.appearance-theme input{height:20px;width:20px}.appearance-swatch{border:1px solid var(--agora-border);display:grid;grid-template-columns:2fr 2fr 1fr;height:54px}.appearance-swatch span{display:block}
  </style><section class="settings-module-panel" data-settings-module="appearance"><p id="appearance-status" role="status">Loading bundled themes…</p><div class="appearance-themes" id="appearance-themes"></div><p class="setting-detail">Preview affects this Settings surface. Apply persists the selected theme; restart shell surfaces when prompted to update all chrome.</p></section>`,
  renderClientController:()=>String.raw`(api)=>{
    const list=api.content.querySelector("#appearance-themes");const status=api.content.querySelector("#appearance-status");const originals=new Map();let previewing=false;
    function restore(){if(!previewing)return;originals.forEach((value,name)=>{if(value)document.documentElement.style.setProperty(name,value);else document.documentElement.style.removeProperty(name);});previewing=false;}
    function preview(theme){restore();Object.entries(theme.tokens).forEach(([name,value])=>{originals.set(name,document.documentElement.style.getPropertyValue(name));document.documentElement.style.setProperty(name,value);});previewing=true;status.textContent="Previewing "+theme.name+"; apply to keep it.";}
    function render(){const state=api.getModuleState(),draft=api.getDraft();if(!state||!draft)return;list.replaceChildren();state.themes.forEach((theme)=>{const label=document.createElement("label");label.className="appearance-theme";const row=document.createElement("span");const input=document.createElement("input");input.type="radio";input.name="appearance-theme";input.value=theme.id;input.checked=draft.themeId===theme.id;const name=document.createElement("strong");name.textContent=theme.name+(state.active.themeId===theme.id?" (active)":"");row.append(input,name);const swatch=document.createElement("span");swatch.className="appearance-swatch";swatch.setAttribute("aria-label",theme.name+" color preview");["--agora-bg","--agora-surface","--agora-accent"].forEach((token)=>{const cell=document.createElement("span");cell.style.background=theme.tokens[token]||"transparent";swatch.appendChild(cell);});input.addEventListener("change",()=>{api.getDraft().themeId=theme.id;api.markDirty();preview(theme);});label.append(row,swatch);list.appendChild(label);});if(!previewing)status.textContent=state.restartRequired?"Theme saved; restart shell surfaces to finish applying it.":"Choose a bundled theme to preview.";}
    return {render,destroy(){restore();}};
  }`,
};
export const appearanceSettingsPageRegistration:SettingsPageRegistration={uiEntryPoint:appearanceSettingsPage.uiEntryPoint,load:async()=>appearanceSettingsPage};
export function assertAppearanceFixtures():void{const theme:AppearanceThemeSummary={id:'agora-default',name:'Agora Tide',tokens:{'--agora-bg':'#001122','--agora-surface':'#112233','--agora-accent':'#44aa99'}};if(themeSwatch(theme).accent!=='#44aa99')throw new Error('appearance swatch must use generated safe tokens');const source=appearanceSettingsPage.renderClientController?.()??'';for(const marker of ['preview(theme)','destroy(){restore()','textContent=theme.name'])if(!source.includes(marker))throw new Error(`appearance controller missing ${marker}`);}
