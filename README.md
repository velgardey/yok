# Yok - Deployment Platform

A platform for building and deploying web applications from Git repositories.

## Architecture

The project consists of three main components:

1. **API Server** (Node.js/Express) - Handles project management and deployment
2. **Reverse Proxy** (Go) - Routes requests to the appropriate deployed applications
3. **Build Server** (Node.js) - Builds applications from Git repositories

## Deployment

The services are deployed on Render:
- API: https://api.yok.ninja
- Reverse Proxy: https://*.yok.ninja

## Keep-Alive Workflow

To prevent the services from sleeping on Render's free tier, a GitHub Actions workflow pings the services every 5 minutes.

### How it works

1. The workflow is defined in `.github/workflows/keep-alive.yml`
2. It runs every 5 minutes using GitHub's scheduled actions
3. It pings both the API and reverse proxy services
4. It logs the status of each service

### Notifications (Optional)

To receive notifications when services are down:

1. Set up a webhook URL (e.g., Slack, Discord, or custom endpoint)
2. Add the webhook URL as a secret named `WEBHOOK_URL` in your GitHub repository settings
3. Uncomment the notification step in the workflow file

## Local Development

To run the services locally:

```bash
docker-compose up
```

## CLI Tool

The Yok CLI is a developer tool for easily deploying web applications to share with others. It combines Git workflow with deployment.

### Installation

#### Linux / macOS
```bash
curl -fsSL https://get.yok.ninja/install.sh | bash
```

#### Windows (PowerShell)
```powershell
iwr -useb https://get.yok.ninja/install.ps1 | iex
```

#### Manual Installation
You can download the appropriate binary for your platform from the [GitHub releases page](https://github.com/velgardey/yok/releases).

### Commands

- **`yok create`** - Create a new project on Yok
- **`yok deploy`** - Deploy your project to the web
- **`yok ship`** - Commit, push, and deploy in one command
- **`yok status [deploymentId]`** - Check deployment status
- **`yok list`** - List all deployments for your project
- **`yok cancel [deploymentId]`** - Cancel a running deployment
- **`yok reset`** - Reset stored project configuration

The CLI also acts as a Git wrapper, allowing you to use standard Git commands:

```bash
yok add .
yok commit -m "Your message"
yok push
```

### Features

- **Real-time Deployment Status**: Monitor deployment progress with live status updates
- **Local/Remote Sync Check**: Ensures your local changes match the remote before deployment
- **Deployment Management**: List, check status, and cancel deployments as needed
- **Interactive UI**: User-friendly prompts and color-coded output
- **Git Integration**: Seamless integration with Git workflow
- **Project Management**: Create and manage projects for deployment

The tool is designed to be simple, making it easy for developers to quickly share their work without dealing with complex deployment processes.

## Environment Variables

The services require several environment variables to be set. See `.env` for the required variables. 