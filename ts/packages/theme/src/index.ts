export const agoraTokenPrefix = '--agora-';

export type ThemeTokenRole =
  | 'presentation'
  | 'layout'
  | 'typography'
  | 'evidence'
  | 'state'
  | 'component';

export interface ShellThemeToken {
  readonly name: `--agora-${string}`;
  readonly value: string;
  readonly role: ThemeTokenRole;
  readonly description: string;
}

export const shellThemeTokens = {
  background: '--agora-bg',
  foreground: '--agora-fg',
  surface: '--agora-surface',
  surfaceRaised: '--agora-surface-raised',
  surfaceStrong: '--agora-surface-strong',
  textMuted: '--agora-text-muted',
  accent: '--agora-accent',
  warning: '--agora-warning',
  border: '--agora-border',
  borderSubtle: '--agora-border-subtle',
  fontFamily: '--agora-font-family',
  fontBackground: '--agora-font-background',
  fontPanel: '--agora-font-panel',
  fontStatus: '--agora-font-status',
  fontCode: '--agora-font-code',
  fontHeading: '--agora-font-heading',
  fontTitle: '--agora-font-title',
  fontBody: '--agora-font-body',
  fontCaption: '--agora-font-caption',
  lineHeightTight: '--agora-line-height-tight',
  lineHeightNormal: '--agora-line-height-normal',
  radiusControl: '--agora-radius-control',
  panelHeight: '--agora-panel-height',
  panelGap: '--agora-panel-gap',
  panelPaddingY: '--agora-panel-padding-y',
  panelPaddingX: '--agora-panel-padding-x',
  controlHeight: '--agora-control-height',
  space1: '--agora-space-1',
  space2: '--agora-space-2',
  space3: '--agora-space-3',
  space4: '--agora-space-4',
  space5: '--agora-space-5',
  duration: '--agora-duration',
  ease: '--agora-ease',
  elevation1: '--agora-elevation-1',
  elevation2: '--agora-elevation-2',
  elevation3: '--agora-elevation-3',
  panelControlHeight: '--agora-panel-control-height',
  panelBackground: '--agora-panel-bg',
  panelShadow: '--agora-panel-shadow',
  popupShadow: '--agora-popup-shadow',
  overlayLabelBackground: '--agora-overlay-label-bg',
  overlayChipBackground: '--agora-overlay-chip-bg',
  focusGlow: '--agora-focus-glow',
  taskbarMinimizedBackground: '--agora-taskbar-minimized-bg',
  taskbarMinimizedBorder: '--agora-taskbar-minimized-border',
  evidenceBackground: '--agora-evidence-bg',
  evidenceAccent: '--agora-evidence-accent',
  evidenceStrong: '--agora-evidence-strong',
} as const;

export type ShellThemeTokenKey = keyof typeof shellThemeTokens;

export const defaultShellThemeTokenDefinitions: readonly ShellThemeToken[] = [
  { name: shellThemeTokens.background, value: '#0b2a39', role: 'presentation', description: 'Canvas background for shell fallback and chrome surfaces.' },
  { name: shellThemeTokens.foreground, value: '#d4e6ec', role: 'presentation', description: 'Primary text color for shell fallback and chrome surfaces.' },
  { name: shellThemeTokens.surface, value: '#143c4c', role: 'presentation', description: 'Panel and plain section background.' },
  { name: shellThemeTokens.surfaceRaised, value: '#1c4a5a', role: 'presentation', description: 'Raised controls, list items, and code blocks.' },
  { name: shellThemeTokens.surfaceStrong, value: '#04202e', role: 'presentation', description: 'High-contrast badges and icon chips.' },
  { name: shellThemeTokens.textMuted, value: '#7090a0', role: 'presentation', description: 'Secondary labels and muted details.' },
  { name: shellThemeTokens.accent, value: '#5ec4a8', role: 'presentation', description: 'Interactive accent for normal shell presentation.' },
  { name: shellThemeTokens.warning, value: '#ffbf3e', role: 'state', description: 'Warning border and status color.' },
  { name: shellThemeTokens.border, value: '#2e5a6a', role: 'presentation', description: 'Default control border.' },
  { name: shellThemeTokens.borderSubtle, value: '#1c4454', role: 'presentation', description: 'Subtle dividers and inactive item borders.' },
  { name: shellThemeTokens.fontFamily, value: 'system-ui, sans-serif', role: 'typography', description: 'Base UI font family.' },
  { name: shellThemeTokens.fontBackground, value: '600 22px var(--agora-font-family)', role: 'typography', description: 'Background shell label font.' },
  { name: shellThemeTokens.fontPanel, value: '600 18px var(--agora-font-family)', role: 'typography', description: 'Panel control font.' },
  { name: shellThemeTokens.fontStatus, value: '600 16px var(--agora-font-family)', role: 'typography', description: 'Operator/status surface font.' },
  { name: shellThemeTokens.fontCode, value: '600 13px ui-monospace, SFMono-Regular, Menlo, monospace', role: 'typography', description: 'Code and recovery command font.' },
  { name: shellThemeTokens.fontHeading, value: '700 24px var(--agora-font-family)', role: 'typography', description: 'Heading font for shell titles and section headers.' },
  { name: shellThemeTokens.fontTitle, value: '700 18px var(--agora-font-family)', role: 'typography', description: 'Title font for cards and surface headers.' },
  { name: shellThemeTokens.fontBody, value: '400 15px var(--agora-font-family)', role: 'typography', description: 'Body text font for list items and descriptions.' },
  { name: shellThemeTokens.fontCaption, value: '500 12px var(--agora-font-family)', role: 'typography', description: 'Caption font for metadata and secondary labels.' },
  { name: shellThemeTokens.lineHeightTight, value: '1.2', role: 'typography', description: 'Tight line height for headings and single-line labels.' },
  { name: shellThemeTokens.lineHeightNormal, value: '1.5', role: 'typography', description: 'Normal line height for body and multi-line text.' },
  { name: shellThemeTokens.radiusControl, value: '4px', role: 'layout', description: 'Shared radius for controls, badges, and app chips.' },
  { name: shellThemeTokens.panelHeight, value: '54px', role: 'layout', description: 'Installed panel height.' },
  { name: shellThemeTokens.panelGap, value: '14px', role: 'layout', description: 'Panel item gap.' },
  { name: shellThemeTokens.panelPaddingY, value: '6px', role: 'layout', description: 'Vertical panel padding.' },
  { name: shellThemeTokens.panelPaddingX, value: '22px', role: 'layout', description: 'Horizontal panel padding.' },
  { name: shellThemeTokens.controlHeight, value: '44px', role: 'layout', description: 'Panel button and compact control height.' },
  { name: shellThemeTokens.space1, value: '4px', role: 'layout', description: 'Smallest spacing step for tight gaps and icon insets.' },
  { name: shellThemeTokens.space2, value: '6px', role: 'layout', description: 'Small spacing step for control clusters.' },
  { name: shellThemeTokens.space3, value: '8px', role: 'layout', description: 'Base spacing step for item gaps and padding.' },
  { name: shellThemeTokens.space4, value: '10px', role: 'layout', description: 'Comfortable spacing step for control padding.' },
  { name: shellThemeTokens.space5, value: '14px', role: 'layout', description: 'Large spacing step for section gaps.' },
  { name: shellThemeTokens.duration, value: '120ms', role: 'component', description: 'Default transition duration for hover and focus states.' },
  { name: shellThemeTokens.ease, value: 'cubic-bezier(0.2, 0.8, 0.2, 1)', role: 'component', description: 'Default easing curve for shell motion.' },
  { name: shellThemeTokens.elevation1, value: '0 1px 2px rgba(0, 0, 0, 0.20)', role: 'component', description: 'Resting elevation for cards and chips.' },
  { name: shellThemeTokens.elevation2, value: '0 6px 18px rgba(0, 0, 0, 0.28)', role: 'component', description: 'Raised elevation for panels and bars.' },
  { name: shellThemeTokens.elevation3, value: '0 18px 60px rgba(0, 0, 0, 0.42)', role: 'component', description: 'Top elevation for popups and overlays.' },
  { name: shellThemeTokens.panelControlHeight, value: '38px', role: 'component', description: 'Taskbar panel control height for dense shell controls.' },
  { name: shellThemeTokens.panelBackground, value: 'color-mix(in srgb, var(--agora-surface) 92%, var(--agora-bg))', role: 'component', description: 'Taskbar panel background treatment.' },
  { name: shellThemeTokens.panelShadow, value: 'inset 0 1px 0 var(--agora-border-subtle), 0 -10px 26px rgba(0, 0, 0, 0.28)', role: 'component', description: 'Taskbar panel top inset and lift shadow.' },
  { name: shellThemeTokens.popupShadow, value: '0 18px 60px rgba(0, 0, 0, 0.42)', role: 'component', description: 'Launcher, status, and WM popup shadow.' },
  { name: shellThemeTokens.overlayLabelBackground, value: 'rgba(8, 13, 30, 0.72)', role: 'component', description: 'Agent overlay label background.' },
  { name: shellThemeTokens.overlayChipBackground, value: 'rgba(8, 13, 30, 0.62)', role: 'component', description: 'Agent overlay compact chip background.' },
  { name: shellThemeTokens.focusGlow, value: '0 0 22px rgba(251, 191, 36, 0.75)', role: 'component', description: 'Focused overlay surface glow.' },
  { name: shellThemeTokens.taskbarMinimizedBackground, value: 'color-mix(in srgb, var(--agora-surface-raised) 74%, var(--agora-bg))', role: 'component', description: 'Taskbar button background for minimized surfaces.' },
  { name: shellThemeTokens.taskbarMinimizedBorder, value: 'color-mix(in srgb, var(--agora-warning) 74%, var(--agora-border))', role: 'component', description: 'Taskbar button border for minimized surfaces.' },
  { name: shellThemeTokens.evidenceBackground, value: '#0f172a', role: 'evidence', description: 'Stable visible-shell evidence background marker; not a general customization token.' },
  { name: shellThemeTokens.evidenceAccent, value: '#00d1b2', role: 'evidence', description: 'Stable visible-shell evidence accent marker used by capture classifiers.' },
  { name: shellThemeTokens.evidenceStrong, value: '#020617', role: 'evidence', description: 'Stable visible-shell evidence strong marker used by capture classifiers.' },
] as const;

export function themeVar(token: ShellThemeTokenKey): `var(${string})` {
  return `var(${shellThemeTokens[token]})`;
}

export function defaultThemeCss(): string {
  const declarations = [...defaultShellThemeTokenDefinitions]
    .sort((left, right) => left.name.localeCompare(right.name))
    .map((token) => `  ${token.name}: ${token.value};`)
    .join('\n');
  return `:root {\n${declarations}\n}`;
}

export const evidenceThemeTokenNames = defaultShellThemeTokenDefinitions
  .filter((token) => token.role === 'evidence')
  .map((token) => token.name);
