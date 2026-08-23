function getBuildCommand(framework) {
  if (framework === 'NEXT') {
    return 'pnpm install && pnpm run build && pnpm dlx next export -o dist';
  }
  return 'pnpm install && pnpm run build';
}

module.exports = { getBuildCommand };
