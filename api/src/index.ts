import { createClient } from '@clickhouse/client';
import { loadConfig } from './config';
import { createKafka, startLogConsumer, type LogMessage } from './bus/kafka';
import { LogStore } from './logs/clickhouse';
import {
  applyStatus,
  failStaleDeployments,
  markFailed,
  promoteLatestDeployment,
} from './services/deploymentService';
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
  const server = app.listen(config.port, () => console.log(`API server listening on port ${config.port}`));

  // Safety net for builds whose compute task died without reporting:
  // anything still active past the timeout is failed.
  const staleSweepMs = config.staleDeploymentMinutes * 60_000;
  const sweeper = setInterval(() => {
    failStaleDeployments(staleSweepMs)
      .then((result) => {
        if (result.count > 0) console.log(`Marked ${result.count} stale deployment(s) as FAILED`);
      })
      .catch((err) => console.error('Stale deployment sweep failed:', err));
  }, 60_000);

  const kafka = createKafka(config.kafka, 'api-server');
  const consumer = await startLogConsumer(kafka, config.kafka.topic, 'api-server-logs-consumer', {
    onLog: (m) => insertLogSafe(logStore, m),
    onStatus: async (m) => {
      if (m.status === 'FAILED') {
        await markFailed(m.deploymentId);
      } else {
        await applyStatus(m.deploymentId, m.status);
      }
      if (m.status === 'COMPLETED') {
        await promoteLatestDeployment(m.projectId, m.deploymentId);
      }
      console.log(`Deployment ${m.deploymentId} -> ${m.status}`);
    },
    onError: (err, raw) => console.error('Bad log message:', err, raw.slice(0, 200)),
  });
  console.log('Kafka log consumer started');

  const shutdown = async (signal: string) => {
    console.log(`Received ${signal}, shutting down...`);
    clearInterval(sweeper);
    server.close();
    await consumer.disconnect().catch(() => undefined);
    process.exit(0);
  };
  process.on('SIGTERM', () => void shutdown('SIGTERM'));
  process.on('SIGINT', () => void shutdown('SIGINT'));
}

main().catch((err) => {
  console.error('Fatal startup error:', err);
  process.exit(1);
});
