import { z } from 'zod';

const envSchema = z.object({
  PORT: z.coerce.number().default(9000),
  CLOUD_PROVIDER: z.enum(['aws']).default('aws'),
  SITE_DOMAIN: z.string().min(1).default('yok.ninja'),
  DATABASE_URL: z.string().min(1),
  KAFKA_BROKER: z.string().min(1),
  KAFKA_USERNAME: z.string().min(1),
  KAFKA_PASSWORD: z.string().min(1),
  KAFKA_TOPIC: z.string().min(1),
  KAFKA_CA_PATH: z.string().optional(),
  CLICKHOUSE_URL: z.string().url(),
  CLICKHOUSE_DATABASE: z.string().min(1),
  PROXY_SERVICE_TOKEN: z.string().min(1),
  BOOTSTRAP_SECRET: z.string().optional(),
});

export interface AppConfig {
  port: number;
  cloudProvider: 'aws';
  siteDomain: string;
  databaseUrl: string;
  kafka: { broker: string; username: string; password: string; topic: string; tlsCaPath?: string };
  clickhouse: { url: string; database: string };
  proxyServiceToken: string;
  bootstrapSecret?: string;
}

export function loadConfig(env: NodeJS.ProcessEnv = process.env): AppConfig {
  const parsed = envSchema.safeParse(env);
  if (!parsed.success) {
    const issues = parsed.error.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join('; ');
    throw new Error(`Invalid configuration: ${issues}`);
  }
  const e = parsed.data;
  return {
    port: e.PORT,
    cloudProvider: e.CLOUD_PROVIDER,
    siteDomain: e.SITE_DOMAIN,
    databaseUrl: e.DATABASE_URL,
    kafka: {
      broker: e.KAFKA_BROKER,
      username: e.KAFKA_USERNAME,
      password: e.KAFKA_PASSWORD,
      topic: e.KAFKA_TOPIC,
      tlsCaPath: e.KAFKA_CA_PATH,
    },
    clickhouse: { url: e.CLICKHOUSE_URL, database: e.CLICKHOUSE_DATABASE },
    proxyServiceToken: e.PROXY_SERVICE_TOKEN,
    bootstrapSecret: e.BOOTSTRAP_SECRET,
  };
}
