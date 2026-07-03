import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';

const root = process.argv[2] ?? process.cwd();
const tsRoot = join(root, 'ts');
const packagesRoot = join(tsRoot, 'packages');
const boundaries = JSON.parse(readFileSync(join(tsRoot, 'boundaries.json'), 'utf8'));
const knownScopes = new Set(boundaries.scopes);
const failures = [];

for (const packageDirName of readdirSync(packagesRoot)) {
  const packageDir = join(packagesRoot, packageDirName);
  if (!statSync(packageDir).isDirectory()) continue;

  const packageJsonPath = join(packageDir, 'package.json');
  const projectJsonPath = join(packageDir, 'project.json');
  const indexPath = join(packageDir, 'src', 'index.ts');

  let packageJson;
  let projectJson;
  try {
    packageJson = JSON.parse(readFileSync(packageJsonPath, 'utf8'));
  } catch {
    failures.push(`missing or invalid package.json: ts/packages/${packageDirName}`);
    continue;
  }
  try {
    projectJson = JSON.parse(readFileSync(projectJsonPath, 'utf8'));
  } catch {
    failures.push(`missing or invalid project.json: ts/packages/${packageDirName}`);
    continue;
  }

  if (packageJson.exports?.['.'] !== './src/index.ts') {
    failures.push(`ts/packages/${packageDirName} must export only the root barrel`);
  }

  try {
    statSync(indexPath);
  } catch {
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

if (failures.length > 0) {
  console.error(failures.join('\n'));
  process.exit(1);
}

console.log('TypeScript structure: OK');

