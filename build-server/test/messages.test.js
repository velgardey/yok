const test = require('node:test');
const assert = require('node:assert');
const { logMessage, statusMessage } = require('../src/bus/kafka');

test('logMessage shapes payload', () => {
  assert.deepEqual(
    logMessage({ projectId: 'p', deploymentId: 'd', log: 'hi' }),
    { type: 'log', projectId: 'p', deploymentId: 'd', log: 'hi' }
  );
});

test('statusMessage whitelists statuses', () => {
  assert.equal(statusMessage({ projectId: 'p', deploymentId: 'd', status: 'COMPLETED' }).status, 'COMPLETED');
  assert.throws(() => statusMessage({ projectId: 'p', deploymentId: 'd', status: 'NOPE' }), /Invalid status/);
});
