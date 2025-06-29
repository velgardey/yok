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
- Reverse Proxy: https://yok.ninja

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

## Environment Variables

The services require several environment variables to be set. See `.env` for the required variables. 