import { createClient } from '@clickhouse/client';
import { loadConfig } from './config';
import { createKafka, startLogConsumer, type LogMessage } from './bus/kafka';
import { LogStore } from './logs/clickhouse';
import { applyStatus, markFailed } from './services/deploymentService';
import { createApp } from './app';

async function insertLogSafe(logStore: LogStore, m: LogMessage): Promise<void> {
  try {
    await logStore.insertLog(m.deploymentId, m.log);
  } catch (err) {
    console.error('Failed to persist log:', err);
  }
}

async function main(): Promise<void> {
  const config = loadConfig();

  const clickhouse = createClient({ url: config.clickhouse.url, database: config.clickhouse.database });
  const logStore = new LogStore(clickhouse);

  const app = createApp(config, logStore);
  app.listen(config.port, () => console.log(`API server listening on port ${config.port}`));

  const kafka = createKafka(config.kafka, 'api-server');
  await startLogConsumer(kafka, config.kafka.topic, 'api-server-logs-consumer', {
    onLog: (m) => insertLogSafe(logStore, m),
    onStatus: async (m) => {
      if (m.status === 'FAILED') {
        await markFailed(m.deploymentId);
      } else {
        await applyStatus(m.deploymentId, m.status);
      }
      console.log(`Deployment ${m.deploymentId} -> ${m.status}`);
    },
    onError: (err, raw) => console.error('Bad log message:', err, raw.slice(0, 200)),
  });
  console.log('Kafka log consumer started');
}

main().catch((err) => {
  console.error('Fatal startup error:', err);
  process.exit(1);
});
