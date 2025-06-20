#!/bin/bash

export GIT_REPO_URL=$GIT_REPO_URL

echo "Cloning repository..." $GIT_REPO_URL
git clone $GIT_REPO_URL /app/output
echo "Repository cloned successfully"

exec node script.js