// Generates the embedded default shell assets consumed by the Go shellAssetHandler
// and catalog icon handler. The TypeScript @agora-de/renderer + @agora-de/components
// packages are the rendering authority; this script projects them to static files
// under go/internal/shellui/server/{shellassets,iconassets} so the Go server can
// go:embed them. Surface HTML templates carry __AGORA_THEME_CSS__ /
// __AGORA_SURFACE__ placeholders that the Go server substitutes at serve time.
//
// Run: node --experimental-strip-types harness/build/generate-shell-html.mjs [out-root]
//
// Default out-root: go/internal/shellui/server  (writes shellassets/ and iconassets/)
import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { appFallbackIconSVG } from '../../ts/packages/components/src/index.ts';
import {
  backgroundHTML,
  launcherHTML,
  operatorHTML,
  overlayHTML,
  panelHTML,
  settingsHTML,
} from '../../ts/packages/renderer/src/shell-html.ts';

const root = process.argv[2] ?? new URL('../../go/internal/shellui/server', import.meta.url).pathname;
const shellAssets = join(root, 'shellassets');
const iconAssets = join(root, 'iconassets');

const files = {
  'panel.html': panelHTML,
  'launcher.html': launcherHTML,
  'operator.html': operatorHTML,
  'settings.html': settingsHTML,
  'overlay.html': overlayHTML,
  'background.html': backgroundHTML({ includeTaskbar: false }),
  'background-fallback.html': backgroundHTML({ includeTaskbar: true }),
};

await mkdir(shellAssets, { recursive: true });
for (const [name, content] of Object.entries(files)) {
  await writeFile(join(shellAssets, name), content, 'utf8');
  console.log(`wrote ${join(shellAssets, name)} (${content.length} bytes)`);
}

// Bundled generic app icon served by the Go catalog icon handler when an app
// icon cannot be resolved from the desktop icon theme.
await mkdir(iconAssets, { recursive: true });
await writeFile(join(iconAssets, 'app-fallback.svg'), appFallbackIconSVG, 'utf8');
console.log(`wrote ${join(iconAssets, 'app-fallback.svg')} (${appFallbackIconSVG.length} bytes)`);
