// Generates the embedded default shell HTML bundle consumed by the Go
// shellAssetHandler. The TypeScript @agora-de/renderer package is the rendering
// authority; this script projects the surface templates to static HTML files
// under go/internal/shellui/server/shellassets so the Go server can go:embed
// them and substitute __AGORA_THEME_CSS__ / __AGORA_SURFACE__ at serve time.
//
// Run: node --experimental-strip-types harness/build/generate-shell-html.mjs [out-dir]
//
// Default out-dir: go/internal/shellui/server/shellassets
import { mkdir, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import {
  backgroundHTML,
  launcherHTML,
  operatorHTML,
  overlayHTML,
  panelHTML,
  settingsHTML,
} from '../../ts/packages/renderer/src/shell-html.ts';

const root = process.argv[2] ?? new URL('../../go/internal/shellui/server/shellassets', import.meta.url).pathname;
const files = {
  'panel.html': panelHTML,
  'launcher.html': launcherHTML,
  'operator.html': operatorHTML,
  'settings.html': settingsHTML,
  'overlay.html': overlayHTML,
  'background.html': backgroundHTML({ includeTaskbar: false }),
  'background-fallback.html': backgroundHTML({ includeTaskbar: true }),
};

await mkdir(root, { recursive: true });
for (const [name, content] of Object.entries(files)) {
  await writeFile(join(root, name), content, 'utf8');
  console.log(`wrote ${join(root, name)} (${content.length} bytes)`);
}
