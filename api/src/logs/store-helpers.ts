import type { LogMessage } from '../bus/kafka';
import type { LogStore } from './clickhouse';

export async function insertLogSafe(logStore: LogStore, m: LogMessage): Promise<void> {
  try {
    await logStore.insertLog(m.deploymentId, m.log);
  } catch (err) {
    console.error('Failed to persist log:', err);
  }
}
