import { describe, expect, it } from 'vitest';
import { parseDeploymentMessage } from './kafka';

describe('parseDeploymentMessage', () => {
  it('parses log messages', () => {
    const m = parseDeploymentMessage(
      JSON.stringify({ type: 'log', projectId: 'p1', deploymentId: 'd1', log: 'hi' })
    );
    expect(m).toEqual({ type: 'log', projectId: 'p1', deploymentId: 'd1', log: 'hi' });
  });

  it('parses valid status messages', () => {
    const m = parseDeploymentMessage(
      JSON.stringify({ type: 'status', projectId: 'p1', deploymentId: 'd1', status: 'IN_PROGRESS' })
    );
    expect(m).toEqual({ type: 'status', projectId: 'p1', deploymentId: 'd1', status: 'IN_PROGRESS' });
  });

  it('rejects invalid status values', () => {
    expect(
      parseDeploymentMessage(JSON.stringify({ type: 'status', projectId: 'p', deploymentId: 'd', status: 'WAT' }))
    ).toBeNull();
  });

  it('returns null on malformed JSON and wrong shapes', () => {
    expect(parseDeploymentMessage('not-json')).toBeNull();
    expect(parseDeploymentMessage('{}')).toBeNull();
    expect(parseDeploymentMessage(JSON.stringify({ type: 'nope' }))).toBeNull();
  });
});
