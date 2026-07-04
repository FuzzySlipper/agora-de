const root = process.argv[2] ?? process.cwd();
const fixtures = await import(`${root}/ts/packages/testing-fixtures/src/index.ts`);

fixtures.assertSurfaceLifecycleFixture();
fixtures.assertAppCatalogVerticalFixture();
fixtures.assertOperatorFeatureFixtures();
fixtures.assertShellRenderClaimFixtures();

console.log('TypeScript fixtures: OK');
