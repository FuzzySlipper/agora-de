import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';

const root = process.argv[2] ?? process.cwd();
const tsRoot = join(root, 'ts');
const packagesRoot = join(tsRoot, 'packages');
const appsRoot = join(tsRoot, 'apps');
const boundaries = JSON.parse(readFileSync(join(tsRoot, 'boundaries.json'), 'utf8'));
const tsconfig = JSON.parse(readFileSync(join(tsRoot, 'tsconfig.json'), 'utf8'));
const knownScopes = new Set(boundaries.scopes);
const failures = [];

const agoraImportPattern = /(?:from\s+|import\s+(?:type\s+)?|import\s*\(\s*)["'](@agora-de\/[a-z0-9-]+)(?:\/[^"']*)?["']/g;
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
    for (const match of text.matchAll(agoraImportPattern)) {
      if (match[1] !== '@agora-de/shell') {
        failures.push(`ts/apps/${appDirName} imports ${match[1]}; apps must import @agora-de/shell only`);
      }
    }
  }
}

if (failures.length > 0) {
  console.error(failures.join('\n'));
  process.exit(1);
}

console.log('TypeScript structure: OK');
