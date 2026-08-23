# Yok - Deployment Platform

A platform for building and deploying web applications from Git repositories.

## Architecture

The project consists of three services plus a CLI:

```
api/                  API server (Node.js/Express + TypeScript)
  src/config.ts       zod-typed environment parsing
  src/routes/         projects, deployments, auth (PAT), resolve, health
  src/services/       project / deployment / token / user logic
  src/middleware/     bearer-token auth middleware
  src/bus/kafka.ts    Kafka consumer wiring
  src/logs/           ClickHouse log store
  src/providers/      ComputeProvider seam; aws/ecs.ts selected via CLOUD_PROVIDER

build-server/         build runner (Node.js, runs as an ECS task)
  src/index.js        orchestration: build -> upload -> status events
  src/bus/kafka.js    Kafka producer publishing log + status events
  src/storage/        storage adapter seam; aws-s3.js selected via STORAGE_PROVIDER

reverse-proxy/        edge router (Go)
  main.go             request handling, slug -> deployment resolution, service auth
  origin.go           OriginResolver seam (S3 today) + per-origin proxy cache

cli/                  Yok CLI (Go/cobra): git wrapper + deploy tool
```

Request flow: the CLI authenticates to the API with a personal access token. Creating a
deployment triggers the configured `ComputeProvider` (an ECS Fargate task today), which
builds the repository and uploads artifacts through the storage adapter while streaming
logs and status events over Kafka into ClickHouse. The reverse proxy authenticates to the
API with `PROXY_SERVICE_TOKEN`, resolves `<slug>.<SITE_DOMAIN>` requests through its
`OriginResolver`, and serves the matching artifact origin.

See [docs/cloud-providers.md](docs/cloud-providers.md) for how to swap any of these pieces
for another cloud provider.

## Hosted instance

- API: https://api.yok.ninja
- Reverse Proxy: https://*.yok.ninja

## CLI Tool

The Yok CLI is a powerful Git wrapper and deployment tool that allows you to deploy your static web applications directly from your Git repository. With Yok, you can quickly share your projects with others without dealing with complex deployment processes.

## Installation

### Linux / macOS

```bash
curl -fsSL https://get.yok.ninja/install.sh | bash
```

### Windows (PowerShell)

```powershell
iwr -useb https://get.yok.ninja/install.ps1 | iex
```

### Manual Installation

1. Download the appropriate binary for your platform from the [GitHub releases page](https://github.com/velgardey/yok/releases).
2. Extract the archive if necessary
3. Move the binary to a location in your PATH:
   - Linux/macOS: `/usr/local/bin` or `~/bin`
   - Windows: Create a folder and add it to your PATH environment variable

To verify your installation, run:

```bash
yok version
```

## Authentication

All API endpoints except `/health` and first-run bootstrap require a Yok personal access
token (PAT, `yok_...`). Tokens are hashed at rest and expire by default after 90 days.

1. **Bootstrap** (operator, first run only): while the database has zero users, create your
   admin user:

   ```bash
   curl -X POST https://api.yok.ninja/auth/bootstrap \
     -H 'Content-Type: application/json' \
     -d '{"email":"you@example.com"}'
   ```

   The response contains the raw token exactly once - save it immediately. Once users exist,
   this endpoint requires the `X-Bootstrap-Secret` header matching `BOOTSTRAP_SECRET`. See
   [docs/cloud-providers.md](docs/cloud-providers.md) for the full bootstrap sequence,
   including creating the reverse proxy service account.

2. **Log in locally**: paste the token into `yok login`. It is validated against `/auth/me`
   and stored in `.yok-config.json` in your project directory (permissions `0600`) alongside
   `apiUrl` and `siteDomain`.

3. **Log out**: `yok logout` clears the stored token.

### CI / non-interactive use

Set environment variables instead of relying on the local config file:

- `YOK_TOKEN` - personal access token used for all API requests
- `YOK_API_URL` - override the API base URL (default: `https://api.yok.ninja`)
- `YOK_SITE_DOMAIN` - override the deployment domain (default: `yok.ninja`)

Environment variables take precedence over the config file.

## Getting Started

To use Yok CLI, navigate to your project directory in the terminal:

```bash
cd path/to/your/project
```

### First-time Setup

1. Log in with your Yok token:

   ```bash
   yok login
   ```

2. Ensure your project is a Git repository. If not, initialize one:

   ```bash
   yok init
   ```

3. Create a new project on Yok:

   ```bash
   yok create
   ```

   You'll be prompted to enter a name for your project and specify how to handle the Git repository (auto-detect or manual entry).

## Commands

### Authentication

#### `yok login`

Authenticates the CLI with your Yok personal access token.

```bash
yok login
```

- Prompts you to paste a `yok_...` token (input is hidden)
- Validates the token against the API (`GET /auth/me`) before saving anything
- Stores the token plus `apiUrl`/`siteDomain` in `.yok-config.json` (`0600`)
- Rejected when the token is invalid, expired or revoked

#### `yok logout`

Clears the stored token from `.yok-config.json`.

```bash
yok logout
```

### Project Management

#### `yok create`

Creates a new project on Yok.

```bash
yok create
```

- You'll be asked to provide a project name
- The tool will check if a project with that name already exists
- You can choose to auto-detect the Git repository from the current directory or manually enter a Git URL
- The framework will be automatically detected based on your project files

#### `yok reset-config`

Resets stored project configuration.

```bash
yok reset-config
```

### Deployment

#### `yok deploy`

Deploys your project to the web using Yok.

```bash
yok deploy [flags]
```

- Checks if your local branch is in sync with the remote
- Handles uncommitted changes if any exist
- Deploys the project and shows real-time deployment status
- Provides the URL where your site is available once deployment completes

Options:
- `-l, --logs`: Follow deployment logs in real-time
- `-n, --no-sync-check`: Skip repository sync check

#### `yok ship`

Commits, pushes, and deploys your project in one command.

```bash
yok ship [flags]
```

- Prompts for a commit message
- Adds all changes, commits them, and pushes to the remote
- Deploys the project and shows real-time deployment status
- Provides the URL where your site is available once deployment completes

Options:
- `-l, --logs`: Follow deployment logs in real-time

### Deployment Management

#### `yok status [deploymentId]`

Checks the status of a deployment.

```bash
yok status
# OR
yok status abc123def
```

- If no deployment ID is provided, you'll be prompted to select from recent deployments
- Shows detailed status information including creation time and last update
- Add the `-l` or `--logs` flag to also view the deployment logs
- Add the `-a` or `--all` flag to show all deployments, not just recent ones

#### `yok logs [deploymentId]`

View and follow logs for a deployment.

```bash
yok logs
# OR
yok logs abc123def
```

- If no deployment ID is provided, you'll be prompted to select from recent deployments
- By default, fetches and displays the logs recorded so far
- Add `-f`/`--follow` to stream new logs as they are generated
- With `--follow` on a running deployment, exits automatically when it completes
- Press Ctrl+C to stop following logs at any time

Options:
- `-f, --follow`: Follow logs as they are generated (default: false)
- `-t, --no-timestamps`: Hide timestamps in log output
- `-c, --no-color`: Disable colored output
- `-r, --raw`: Display raw log output without formatting
- `-w, --wait`: Wait for completion and exit automatically when logs are complete (default: false)

#### `yok list`

Lists all deployments for your project.

```bash
yok list
```

- Displays a table with deployment IDs, statuses, and creation times
- Color-coded statuses for easy identification

#### `yok cancel [deploymentId]`

Cancels a running deployment.

```bash
yok cancel
# OR
yok cancel abc123def
```

- If no deployment ID is provided, you'll be prompted to select from in-progress deployments
- Requires confirmation before cancellation

### Git Integration

Yok CLI acts as a Git wrapper, allowing you to use standard Git commands:

```bash
yok add .
yok commit -m "Your message"
yok push
yok pull
yok checkout -b new-branch
yok branch
yok status
yok log
# and many more
```

All standard Git commands are supported, making Yok a seamless part of your Git workflow.

## Features

### Real-time Deployment Status

Monitor deployment progress with live status updates. The CLI will automatically follow the deployment process and notify you when it completes or fails.

### Local/Remote Sync Check

Before deployment, Yok checks if your local repository is in sync with the remote:
- Detects if you're behind or ahead of the remote
- Identifies uncommitted changes
- Offers to commit and push changes before deploying

### Interactive UI

- User-friendly prompts for all necessary inputs
- Color-coded output for better readability
- Spinners to indicate ongoing operations

### Project Management

- Create and manage projects for deployment
- Projects are linked to Git repositories
- Framework auto-detection for optimal deployment settings

### Custom Domains

Once deployed, your site will be available at:
- `https://[project-slug].yok.ninja`
- A unique deployment URL for each deployment

## Configuration

All runtime configuration flows through environment variables; committed examples list every
key each component reads:

- [`.env.example`](.env.example) - compose-level file consumed by `docker-compose.yaml`
- [`api/.env.example`](api/.env.example) - API server subset
- [`build-server/.env.example`](build-server/.env.example) - build task subset
- [`reverse-proxy/.env.example`](reverse-proxy/.env.example) - reverse proxy subset

Provider selection is env-driven: `CLOUD_PROVIDER` picks the API's compute provider,
`STORAGE_PROVIDER` picks the build server's artifact storage adapter, and
`ARTIFACT_BASE_URL` points the reverse proxy at the artifact origin. To run Yok on another
cloud (e.g. Azure), see [docs/cloud-providers.md](docs/cloud-providers.md).

## Troubleshooting

### Common Issues

1. **"No project configured"**
   - Run `yok create` to set up a project

2. **"Failed to check if behind/ahead of remote"**
   - Ensure your Git repository has a remote set up
   - Run `git remote -v` to verify

3. **"You have uncommitted changes"**
   - Commit your changes with `yok ship` or
   - Use `yok deploy` and follow the prompts to handle uncommitted changes

4. **"Failed to deploy project"**
   - Check your internet connection
   - Verify your Git repository is accessible

5. **"401 Unauthorized" / "Invalid or expired token"**
   - Your stored token is missing, expired or revoked - run `yok login` again
   - In CI, check that `YOK_TOKEN` is set and has not expired (tokens expire after 90 days
     unless created with a custom expiry)

