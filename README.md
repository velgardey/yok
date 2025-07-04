# Yok - Deployment Platform

A platform for building and deploying web applications from Git repositories.

## Architecture

The project consists of three main components:

1. **API Server** (Node.js/Express) - Handles project management and deployment
2. **Reverse Proxy** (Go) - Routes requests to the appropriate deployed applications
3. **Build Server** (Node.js) - Builds applications from Git repositories

## Deployment

The services are deployed on:
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

## Getting Started

To use Yok CLI, navigate to your project directory in the terminal:

```bash
cd path/to/your/project
```

### First-time Setup

1. Ensure your project is a Git repository. If not, initialize one:

   ```bash
   yok init
   ```

2. Create a new project on Yok:

   ```bash
   yok create
   ```

   You'll be prompted to enter a name for your project and specify how to handle the Git repository (auto-detect or manual entry).

## Commands

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

#### `yok logs [deploymentId]`

View and follow logs for a deployment.

```bash
yok logs
# OR
yok logs abc123def
```

- If no deployment ID is provided, you'll be prompted to select from recent deployments
- Shows real-time logs as they are generated
- Automatically exits when the deployment completes
- Press Ctrl+C to stop following logs at any time

Options:
- `-f, --follow`: Follow logs as they are generated (default: true)
- `-t, --no-timestamps`: Hide timestamps in log output
- `-c, --no-color`: Disable colored output
- `-r, --raw`: Display raw log output without formatting
- `-w, --wait`: Wait for completion and exit automatically when logs are complete (default: true)

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

