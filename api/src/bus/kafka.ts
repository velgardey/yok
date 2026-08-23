import { readFileSync } from 'fs';
import { Kafka, Partitioners, type Consumer } from 'kafkajs';
import type { AppConfig } from '../config';

const STATUSES = ['PENDING', 'QUEUED', 'IN_PROGRESS', 'COMPLETED', 'FAILED'] as const;
export type DeploymentStatus = (typeof STATUSES)[number];

export interface LogMessage {
  type: 'log';
  projectId: string;
  deploymentId: string;
  log: string;
}

export interface StatusMessage {
  type: 'status';
  projectId: string;
  deploymentId: string;
  status: DeploymentStatus;
}

export type DeploymentMessage = LogMessage | StatusMessage;

export function parseDeploymentMessage(raw: string): DeploymentMessage | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof parsed !== 'object' || parsed === null) return null;
  const m = parsed as Record<string, unknown>;
  if (typeof m.projectId !== 'string' || typeof m.deploymentId !== 'string') return null;

  if (m.type === 'log' && typeof m.log === 'string') {
    return { type: 'log', projectId: m.projectId, deploymentId: m.deploymentId, log: m.log };
  }
  if (m.type === 'status' && typeof m.status === 'string' && STATUSES.includes(m.status as never)) {
    return { type: 'status', projectId: m.projectId, deploymentId: m.deploymentId, status: m.status as DeploymentStatus };
  }
  return null;
}

export function createKafka(kafkaCfg: AppConfig['kafka'], clientId: string): Kafka {
  return new Kafka({
    clientId,
    brokers: [kafkaCfg.broker],
    sasl: { username: kafkaCfg.username, password: kafkaCfg.password, mechanism: 'plain' },
    ssl: kafkaCfg.tlsCaPath ? { ca: [readFileSync(kafkaCfg.tlsCaPath, 'utf-8')] } : true,
  });
}

export interface LogHandlers {
  onLog(m: LogMessage): Promise<void>;
  onStatus(m: StatusMessage): Promise<void>;
  onError(err: unknown, raw: string): void;
}

export async function startLogConsumer(
  kafka: Kafka,
  topic: string,
  groupId: string,
  handlers: LogHandlers
): Promise<Consumer> {
  const consumer = kafka.consumer({ groupId });
  await consumer.connect();
  await consumer.subscribe({ topic, fromBeginning: false });

  await consumer.run({
    eachBatch: async ({ batch, heartbeat, commitOffsetsIfNecessary, resolveOffset }) => {
      for (const message of batch.messages) {
        if (!message.value) continue;
        const raw = message.value.toString();
        try {
          const msg = parseDeploymentMessage(raw);
          if (!msg) {
            handlers.onError(new Error('Unrecognized message shape'), raw);
          } else if (msg.type === 'log') {
            await handlers.onLog(msg);
          } else {
            await handlers.onStatus(msg);
          }
          resolveOffset(message.offset);
        } catch (err) {
          handlers.onError(err, raw);
        }
      }
      await commitOffsetsIfNecessary();
      await heartbeat();
    },
  });
  return consumer;
}

export function createProducer(kafka: Kafka) {
  return kafka.producer({ createPartitioner: Partitioners.DefaultPartitioner, allowAutoTopicCreation: false });
}
