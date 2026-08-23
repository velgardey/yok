const test = require('node:test');
const assert = require('node:assert');
const { getBuildCommand } = require('../src/build');

test('NEXT gets export step', () => {
  assert.match(getBuildCommand('NEXT'), /next export/);
});

test('other frameworks get plain build', () => {
  for (const fw of ['REACT', 'VITE', 'VUE', 'ANGULAR', 'SVELTE', 'OTHER']) {
    assert.equal(getBuildCommand(fw), 'pnpm install && pnpm run build');
  }
});
