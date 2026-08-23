const VALID_STATUSES = ['PENDING', 'QUEUED', 'IN_PROGRESS', 'COMPLETED', 'FAILED'];

function getBuildCommand(framework) {
  if (framework === 'NEXT') {
    return 'pnpm install && pnpm run build && pnpm dlx next export -o dist';
  }
  return 'pnpm install && pnpm run build';
}

module.exports = { getBuildCommand, VALID_STATUSES, KNOWN_OUTPUT_DIRS: ['dist'] };
