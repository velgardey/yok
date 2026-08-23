import { randomUUID } from 'crypto';
import type { ClickHouseClient } from '@clickhouse/client';

export interface LogRow {
  event_id: string;
  deployment_id: string;
  log: string;
  timestamp: string;
}

export class LogStore {
  constructor(private readonly client: ClickHouseClient) {}

  async insertLog(deploymentId: string, log: string): Promise<void> {
    await this.client.insert({
      table: 'log_events',
      values: [{ event_id: randomUUID(), deployment_id: deploymentId, log }],
      format: 'JSONEachRow',
    });
  }

  async listByDeployment(deploymentId: string): Promise<LogRow[]> {
    const result = await this.client.query({
      query:
        'SELECT event_id, deployment_id, log, timestamp FROM log_events WHERE deployment_id = {deployment_id:String} ORDER BY timestamp ASC',
      query_params: { deployment_id: deploymentId },
      format: 'JSONEachRow',
    });
    return (await result.json()) as LogRow[];
  }
}
