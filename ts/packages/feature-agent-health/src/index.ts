import type { FeatureManifest } from '@agora-de/domain';

export const agentHealthFeatureManifest = {
  id: 'feature-agent-health',
  title: 'Agent Health',
  surfaces: ['operator-console'],
} as const satisfies FeatureManifest;

export const featureAgentHealth = agentHealthFeatureManifest.id;
