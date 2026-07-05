export const agoraTokenPrefix = '--agora-';

export type ThemeTokenRole = 'presentation' | 'layout' | 'typography' | 'evidence' | 'state';

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
  radiusControl: '--agora-radius-control',
  panelHeight: '--agora-panel-height',
  panelGap: '--agora-panel-gap',
  panelPaddingX: '--agora-panel-padding-x',
  controlHeight: '--agora-control-height',
  evidenceBackground: '--agora-evidence-bg',
  evidenceAccent: '--agora-evidence-accent',
  evidenceStrong: '--agora-evidence-strong',
} as const;

export type ShellThemeTokenKey = keyof typeof shellThemeTokens;

export const defaultShellThemeTokenDefinitions: readonly ShellThemeToken[] = [
  { name: shellThemeTokens.background, value: '#0f172a', role: 'presentation', description: 'Canvas background for shell fallback and chrome surfaces.' },
  { name: shellThemeTokens.foreground, value: '#e5eef7', role: 'presentation', description: 'Primary text color for shell fallback and chrome surfaces.' },
  { name: shellThemeTokens.surface, value: '#111827', role: 'presentation', description: 'Panel and plain section background.' },
  { name: shellThemeTokens.surfaceRaised, value: '#1f2937', role: 'presentation', description: 'Raised controls, list items, and code blocks.' },
  { name: shellThemeTokens.surfaceStrong, value: '#020617', role: 'presentation', description: 'High-contrast badges and icon chips.' },
  { name: shellThemeTokens.textMuted, value: '#94a3b8', role: 'presentation', description: 'Secondary labels and muted details.' },
  { name: shellThemeTokens.accent, value: '#5eead4', role: 'presentation', description: 'Interactive accent for normal shell presentation.' },
  { name: shellThemeTokens.warning, value: '#fbbf24', role: 'state', description: 'Warning border and status color.' },
  { name: shellThemeTokens.border, value: '#64748b', role: 'presentation', description: 'Default control border.' },
  { name: shellThemeTokens.borderSubtle, value: '#334155', role: 'presentation', description: 'Subtle dividers and inactive item borders.' },
  { name: shellThemeTokens.fontFamily, value: 'system-ui, sans-serif', role: 'typography', description: 'Base UI font family.' },
  { name: shellThemeTokens.fontBackground, value: '600 22px var(--agora-font-family)', role: 'typography', description: 'Background shell label font.' },
  { name: shellThemeTokens.fontPanel, value: '600 18px var(--agora-font-family)', role: 'typography', description: 'Panel control font.' },
  { name: shellThemeTokens.fontStatus, value: '600 16px var(--agora-font-family)', role: 'typography', description: 'Operator/status surface font.' },
  { name: shellThemeTokens.fontCode, value: '600 13px ui-monospace, SFMono-Regular, Menlo, monospace', role: 'typography', description: 'Code and recovery command font.' },
  { name: shellThemeTokens.radiusControl, value: '4px', role: 'layout', description: 'Shared radius for controls, badges, and app chips.' },
  { name: shellThemeTokens.panelHeight, value: '96px', role: 'layout', description: 'Installed panel height.' },
  { name: shellThemeTokens.panelGap, value: '14px', role: 'layout', description: 'Panel item gap.' },
  { name: shellThemeTokens.panelPaddingX, value: '22px', role: 'layout', description: 'Horizontal panel padding.' },
  { name: shellThemeTokens.controlHeight, value: '44px', role: 'layout', description: 'Panel button and compact control height.' },
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
