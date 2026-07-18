const root = process.argv[2] ?? process.cwd();
const fixtures = await import(`${root}/ts/packages/testing-fixtures/src/index.ts`);
const settingsFixtures = await import(`${root}/ts/packages/settings-testing-fixtures/src/index.ts`);

fixtures.assertSurfaceLifecycleFixture();
fixtures.assertAppCatalogVerticalFixture();
fixtures.assertOperatorFeatureFixtures();
fixtures.assertShellRenderClaimFixtures();
fixtures.assertThemeFixture();
fixtures.assertSettingsContractFixtures();
await settingsFixtures.assertSettingsAuthoringKitFixtures();

console.log('TypeScript fixtures: OK');
