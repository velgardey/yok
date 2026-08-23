#!/bin/bash
set -euo pipefail

echo "Cloning repository..."
git clone "$GIT_REPO_URL" /app/output
echo "Repository cloned successfully"

exec node src/index.js
