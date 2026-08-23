const test = require('node:test');
const assert = require('node:assert');
const { getBuildCommand } = require('../src/build');

test('all frameworks get the plain build command', () => {
  for (const fw of ['NEXT', 'REACT', 'VITE', 'VUE', 'ANGULAR', 'SVELTE', 'OTHER', 'STATIC']) {
    assert.equal(getBuildCommand(fw), 'pnpm install && pnpm run build');
  }
});
