import type { AppConfig } from '../config';
import { EcsComputeProvider, ecsConfigFromEnv } from './aws/ecs';
import type { ComputeProvider } from './types';

export function getComputeProvider(config: AppConfig): ComputeProvider {
  switch (config.cloudProvider) {
    case 'aws':
      return new EcsComputeProvider(ecsConfigFromEnv());
    default:
      throw new Error(`Unsupported CLOUD_PROVIDER: ${String(config.cloudProvider)}`);
  }
}
