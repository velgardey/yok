import { describe, expect, it } from 'vitest';
import { getComputeProvider } from './factory';
import type { AppConfig } from '../config';

const baseCfg = {
  port: 9000, cloudProvider: 'aws', siteDomain: 'x.ninja',
  databaseUrl: 'postgres://x', proxyServiceToken: 't', bootstrapSecret: undefined,
  staleDeploymentMinutes: 30,
  kafka: { broker: 'b', username: 'u', password: 'p', topic: 't' },
  clickhouse: { url: 'https://c', database: 'd' },
} satisfies AppConfig;

describe('getComputeProvider', () => {
  it('returns ECS provider for aws', () => {
    expect(getComputeProvider(baseCfg)).toBeTruthy();
  });

  it('throws for unsupported provider', () => {
    expect(() => getComputeProvider({ ...baseCfg, cloudProvider: 'azure' as never })).toThrow(/Unsupported/);
  });
});
