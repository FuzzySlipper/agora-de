import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';

const root = process.argv[2] ?? process.cwd();
const tsRoot = join(root, 'ts');
const packagesRoot = join(tsRoot, 'packages');
const appsRoot = join(tsRoot, 'apps');
const boundaries = JSON.parse(readFileSync(join(tsRoot, 'boundaries.json'), 'utf8'));
const tsconfig = JSON.parse(readFileSync(join(tsRoot, 'tsconfig.json'), 'utf8'));
const knownScopes = new Set(boundaries.scopes);
const failures = [];

const importPattern = /(?:from\s+|import\s+(?:type\s+)?|import\s+|import\s*\(\s*)["']([^"']+)["']/g;
const agoraImportPattern = /^(?<packageName>@agora-de\/[a-z0-9-]+)(?<suffix>\/.*)?$/;
const rootReferencePaths = new Set((tsconfig.references ?? []).map((reference) => reference.path));

function readJson(path, label) {
  try {
    return JSON.parse(readFileSync(path, 'utf8'));
  } catch {
    failures.push(`missing or invalid ${label}`);
    return undefined;
  }
}

function exists(path) {
  try {
    statSync(path);
    return true;
  } catch {
    return false;
  }
}

function collectTypeScriptFiles(dir) {
  const files = [];
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    const stat = statSync(path);
    if (stat.isDirectory()) {
      files.push(...collectTypeScriptFiles(path));
    } else if (entry.endsWith('.ts')) {
      files.push(path);
    }
  }
  return files;
}

function repoPath(path) {
  return relative(root, path);
}

function checkGeneratedContractImport(source, specifier) {
  if (!specifier.includes('generated/contracts') && !specifier.includes('protocol/src/generated')) {
    return;
  }

  if (repoPath(source) === 'ts/packages/protocol/src/index.ts' && specifier === './generated/contracts.js') {
    return;
  }

  failures.push(`${repoPath(source)} imports generated contracts directly; use @agora-de/protocol`);
}

function settingsSiblingFeatureViolation(sourcePackage, specifier) {
  if (!sourcePackage.startsWith('feature-settings-')) return false;
  if (!specifier.startsWith('@agora-de/feature-settings-')) return false;
  return specifier !== `@agora-de/${sourcePackage}`;
}

for (const packageDirName of readdirSync(packagesRoot)) {
  const packageDir = join(packagesRoot, packageDirName);
  if (!statSync(packageDir).isDirectory()) continue;

  const packageJsonPath = join(packageDir, 'package.json');
  const projectJsonPath = join(packageDir, 'project.json');
  const indexPath = join(packageDir, 'src', 'index.ts');

  const packageJson = readJson(packageJsonPath, `package.json: ts/packages/${packageDirName}`);
  const projectJson = readJson(projectJsonPath, `project.json: ts/packages/${packageDirName}`);
  if (!packageJson || !projectJson) continue;

  if (packageJson.exports?.['.'] !== './src/index.ts') {
    failures.push(`ts/packages/${packageDirName} must export only the root barrel`);
  }

  if (!exists(indexPath)) {
    failures.push(`ts/packages/${packageDirName} must expose src/index.ts`);
  }

  const tags = new Set(projectJson.tags ?? []);
  const typeTags = [...tags].filter((tag) => tag.startsWith('type:'));
  const scopeTags = [...tags].filter((tag) => tag.startsWith('scope:'));
  if (typeTags.length !== 1) failures.push(`ts/packages/${packageDirName} needs exactly one type: tag`);
  if (scopeTags.length !== 1) failures.push(`ts/packages/${packageDirName} needs exactly one scope: tag`);
  const scope = scopeTags[0]?.slice('scope:'.length);
  if (scope && !knownScopes.has(scope)) failures.push(`ts/packages/${packageDirName} has unknown scope ${scope}`);

  const sourceDir = join(packageDir, 'src');
  if (!exists(sourceDir)) continue;
  for (const source of collectTypeScriptFiles(sourceDir)) {
    const text = readFileSync(source, 'utf8');
    for (const match of text.matchAll(importPattern)) {
      const specifier = match[1];
      checkGeneratedContractImport(source, specifier);
      if (settingsSiblingFeatureViolation(packageDirName, specifier)) {
        failures.push(`ts/packages/${packageDirName} imports sibling settings feature ${specifier}`);
      }
    }
  }
}

const siblingFixture = readJson(
  join(root, 'harness', 'fixtures', 'depgraph', 'settings-sibling-feature-import.json'),
  'settings sibling feature import fixture',
);
if (
  siblingFixture &&
  !settingsSiblingFeatureViolation(siblingFixture.sourcePackage, siblingFixture.specifier)
) {
  failures.push('settings sibling feature import regression fixture was not rejected');
}

const productionComposition = [
  join(root, 'harness', 'build', 'generate-shell-html.mjs'),
  ...collectTypeScriptFiles(join(root, 'ts', 'packages', 'shell', 'src')),
];
for (const source of productionComposition) {
  if (readFileSync(source, 'utf8').includes('@agora-de/settings-testing-fixtures')) {
    failures.push(`${repoPath(source)} includes the non-production settings fixture module`);
  }
}

for (const appDirName of readdirSync(appsRoot)) {
  const appDir = join(appsRoot, appDirName);
  if (!statSync(appDir).isDirectory()) continue;

  const tsconfig = readJson(join(appDir, 'tsconfig.json'), `tsconfig.json: ts/apps/${appDirName}`);
  if (!exists(join(appDir, 'src', 'main.ts'))) {
    failures.push(`ts/apps/${appDirName} must expose src/main.ts`);
  }
  if (!rootReferencePaths.has(`apps/${appDirName}`)) {
    failures.push(`ts/tsconfig.json must reference apps/${appDirName}`);
  }

  const references = tsconfig?.references ?? [];
  const referencePaths = new Set(references.map((reference) => reference.path));
  if (referencePaths.size !== 1 || !referencePaths.has('../../packages/shell')) {
    failures.push(`ts/apps/${appDirName} must reference only ../../packages/shell`);
  }

  const sourceDir = join(appDir, 'src');
  if (!exists(sourceDir)) continue;

  for (const source of collectTypeScriptFiles(sourceDir)) {
    const text = readFileSync(source, 'utf8');
    for (const match of text.matchAll(importPattern)) {
      const specifier = match[1];
      checkGeneratedContractImport(source, specifier);
      const agoraImport = specifier.match(agoraImportPattern);
      if (agoraImport) {
        const { packageName, suffix } = agoraImport.groups;
        if (packageName !== '@agora-de/shell' || suffix) {
          failures.push(`ts/apps/${appDirName} imports ${specifier}; apps must import @agora-de/shell only`);
        }
      }
    }
  }
}

if (failures.length > 0) {
  console.error(failures.join('\n'));
  process.exit(1);
}

console.log('TypeScript structure: OK');
