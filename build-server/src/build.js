function getBuildCommand() {
  // Every framework builds with the project's own build script; the output
  // directory is detected afterwards (see findOutputDir in src/index.js).
  // Note: Next.js apps need `output: 'export'` in next.config to emit a
  // static `out/` directory - there is no CLI flag for it anymore.
  return 'pnpm install && pnpm run build';
}

module.exports = { getBuildCommand };
