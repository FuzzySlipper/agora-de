import { themeVar } from '@agora-de/theme';

export interface ComponentMarker {
  readonly visualId: string;
}

// Shared symbolic SVG icon set for shell chrome controls. Control glyphs are
// stroke-based and use currentColor so they re-color with theme tokens
// (var(--agora-fg) / var(--agora-accent)). The app-fallback glyph is
// self-colored because it is served as an <img> for unresolved app icons.
export interface ShellIcon {
  readonly paths: string;
}

export const shellIcons = {
  refresh: {
    paths:
      '<path d="M4 12a8 8 0 0 1 13.7-5.7L20 8" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2"></path>' +
      '<path d="M20 4v4h-4" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2"></path>' +
      '<path d="M20 12a8 8 0 0 1-13.7 5.7L4 16" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2"></path>' +
      '<path d="M4 20v-4h4" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2"></path>',
  },
  status: {
    paths:
      '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2"></circle>' +
      '<path d="M12 8h.01" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="3"></path>' +
      '<path d="M11 12h1v5h1" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2"></path>',
  },
  settings: {
    paths:
      '<circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" stroke-width="2"></circle>' +
      '<path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1A2 2 0 1 1 4.2 17l.1-.1A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-1.6-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.3 7A2 2 0 1 1 7.1 4.2l.1.1A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-1.6V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1A2 2 0 1 1 19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1Z" fill="none" stroke="currentColor" stroke-linejoin="round" stroke-width="2"></path>',
  },
  close: {
    paths:
      '<path d="M6 6l12 12M18 6L6 18" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path>',
  },
  minimize: {
    paths:
      '<path d="M5 12h14" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path>',
  },
  maximize: {
    paths:
      '<rect x="5" y="5" width="14" height="14" rx="2" fill="none" stroke="currentColor" stroke-width="2"></rect>',
  },
  workspaces: {
    paths:
      '<rect x="4" y="4" width="7" height="7" rx="1.5" fill="none" stroke="currentColor" stroke-width="2"></rect>' +
      '<rect x="13" y="4" width="7" height="7" rx="1.5" fill="none" stroke="currentColor" stroke-width="2"></rect>' +
      '<rect x="4" y="13" width="7" height="7" rx="1.5" fill="none" stroke="currentColor" stroke-width="2"></rect>' +
      '<rect x="13" y="13" width="7" height="7" rx="1.5" fill="none" stroke="currentColor" stroke-width="2"></rect>',
  },
  layout: {
    paths:
      '<rect x="4" y="4" width="16" height="16" rx="2" fill="none" stroke="currentColor" stroke-width="2"></rect>' +
      '<path d="M4 10h16M10 10v10" fill="none" stroke="currentColor" stroke-width="2"></path>',
  },
  search: {
    paths:
      '<circle cx="11" cy="11" r="6" fill="none" stroke="currentColor" stroke-width="2"></circle>' +
      '<path d="M20 20l-4-4" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2"></path>',
  },
} as const satisfies Record<string, ShellIcon>;

export type ShellIconName = keyof typeof shellIcons;

export function shellIconSVG(name: ShellIconName, className = 'agora-icon'): string {
  return `<svg class="${className}" viewBox="0 0 24 24" aria-hidden="true">${shellIcons[name].paths}</svg>`;
}

export const appFallbackIconSVG =
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="48" height="48">' +
  '<rect x="2" y="2" width="20" height="20" rx="4" fill="#1f2937" stroke="#64748b" stroke-width="1.5"/>' +
  '<circle cx="8" cy="8" r="1.6" fill="#94a3b8"/>' +
  '<circle cx="16" cy="8" r="1.6" fill="#94a3b8"/>' +
  '<circle cx="8" cy="16" r="1.6" fill="#94a3b8"/>' +
  '<circle cx="16" cy="16" r="1.6" fill="#94a3b8"/>' +
  '</svg>';


// Token-driven theme var bindings for shared shell primitives. Feature
// libraries and surface templates consume these instead of inventing unrelated
// visual constants (per docs/theme-boundary.md).
export const componentThemeVars = {
  controlHeight: themeVar('controlHeight'),
  panelControlHeight: themeVar('panelControlHeight'),
  radiusControl: themeVar('radiusControl'),
  space1: themeVar('space1'),
  space2: themeVar('space2'),
  space3: themeVar('space3'),
  space4: themeVar('space4'),
  space5: themeVar('space5'),
  duration: themeVar('duration'),
  ease: themeVar('ease'),
  elevation1: themeVar('elevation1'),
  elevation2: themeVar('elevation2'),
  elevation3: themeVar('elevation3'),
  surface: themeVar('surface'),
  surfaceRaised: themeVar('surfaceRaised'),
  surfaceStrong: themeVar('surfaceStrong'),
  border: themeVar('border'),
  borderSubtle: themeVar('borderSubtle'),
  foreground: themeVar('foreground'),
  accent: themeVar('accent'),
} as const;

// Shared shell primitive base stylesheet. Inlined into every surface <style>
// by @agora-de/renderer. This is the single source for button, badge, chip,
// surface, card, and popup base treatments; surface templates define only
// variants on top of these. No hardcoded colors, radii, or dimensions — every
// value routes through var(--agora-*).
export const componentCSS = `    button {
      font: inherit;
    }
    button:disabled {
      opacity: 0.55;
    }
    .visually-hidden {
      clip: rect(0 0 0 0);
      clip-path: inset(50%);
      height: 1px;
      overflow: hidden;
      position: absolute;
      white-space: nowrap;
      width: 1px;
    }
    .agora-control {
      align-items: center;
      border-radius: var(--agora-radius-control);
      box-sizing: border-box;
      display: inline-flex;
      font: var(--agora-font-body);
      height: var(--agora-control-height);
      justify-content: center;
      line-height: var(--agora-line-height-tight);
      padding: 0 var(--agora-space-3);
      transition: border-color var(--agora-duration) var(--agora-ease),
        background var(--agora-duration) var(--agora-ease),
        color var(--agora-duration) var(--agora-ease);
    }
    .agora-control:disabled {
      opacity: 0.55;
    }
    .chip,
    .badge {
      align-items: center;
      background: var(--agora-surface-strong);
      border-radius: var(--agora-radius-control);
      color: var(--agora-fg);
      display: inline-flex;
      font: var(--agora-font-caption);
      gap: var(--agora-space-2);
      height: var(--agora-control-height);
      justify-content: center;
      line-height: var(--agora-line-height-tight);
      padding: 0 var(--agora-space-4);
    }
    .surface,
    .card {
      background: var(--agora-surface);
      border: 2px solid var(--agora-border-subtle);
      border-radius: var(--agora-radius-control);
      box-shadow: var(--agora-elevation-1);
      box-sizing: border-box;
      font: var(--agora-font-body);
      line-height: var(--agora-line-height-normal);
    }
    .agora-popup {
      background: var(--agora-surface-raised);
      border: 2px solid var(--agora-border);
      border-radius: var(--agora-radius-control);
      box-shadow: var(--agora-elevation-3);
      box-sizing: border-box;
    }`;
