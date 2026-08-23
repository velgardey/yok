import { describe, expect, it } from 'vitest';
import { loadConfig } from './config';

const validEnv = {
  PORT: '9000',
  CLOUD_PROVIDER: 'aws',
  DATABASE_URL: 'postgres://u:p@h/db',
  KAFKA_BROKER: 'b:9092',
  KAFKA_USERNAME: 'u',
  KAFKA_PASSWORD: 'p',
  KAFKA_TOPIC: 'logs',
  CLICKHOUSE_URL: 'https://ch.example.com',
  CLICKHOUSE_DATABASE: 'logs',
  PROXY_SERVICE_TOKEN: 'yok_proxy',
  SITE_DOMAIN: 'yok.ninja',
};

describe('loadConfig', () => {
  it('parses a complete environment', () => {
    const cfg = loadConfig(validEnv);
    expect(cfg.port).toBe(9000);
    expect(cfg.cloudProvider).toBe('aws');
    expect(cfg.kafka.topic).toBe('logs');
    expect(cfg.siteDomain).toBe('yok.ninja');
    expect(cfg.bootstrapSecret).toBeUndefined();
  });

  it('applies defaults', () => {
    const cfg = loadConfig({ ...validEnv, PORT: undefined, SITE_DOMAIN: undefined });
    expect(cfg.port).toBe(9000);
    expect(cfg.siteDomain).toBe('yok.ninja');
  });

  it('rejects missing required variables', () => {
    const { DATABASE_URL: _drop, ...incomplete } = validEnv;
    expect(() => loadConfig(incomplete)).toThrow();
  });

  it('rejects unknown cloud provider', () => {
    expect(() => loadConfig({ ...validEnv, CLOUD_PROVIDER: 'gcp' })).toThrow();
  });
});
