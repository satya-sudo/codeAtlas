# API Gateway

Minimal reverse proxy for local development and public exposure through a single `ngrok` tunnel.

Default port: `8060`

Current routes:

- `/auth/*` -> auth-service
- `/repos` and `/repos/*` -> repo-service
- `/integrations/github/*` -> repo-service
- `/webhooks/github/*` -> webhook-service

## Why it exists

With one free `ngrok` tunnel, exposing each service separately becomes awkward. The gateway gives CodeAtlas one stable public base URL for:

- GitHub OAuth callback
- GitHub App setup callback
- future GitHub webhook delivery
- frontend API calls

## Local service targets

- `AUTH_SERVICE_BASE_URL=http://localhost:8061`
- `REPO_SERVICE_BASE_URL=http://localhost:8062`
- `WEBHOOK_SERVICE_BASE_URL=http://localhost:8063`

## Ngrok setup

Point `ngrok` to the gateway instead of an individual backend service:

```bash
ngrok http 8060
```

If your public tunnel is:

```text
https://your-subdomain.ngrok-free.dev
```

then use that single base URL everywhere below.

## GitHub OAuth values

- GitHub OAuth callback URL:
  - `https://your-subdomain.ngrok-free.dev/auth/github/callback`

The auth-service should also use the same value in:

- `GITHUB_REDIRECT_URL`

## GitHub App values

- Post-install setup URL:
  - `https://your-subdomain.ngrok-free.dev/integrations/github/setup`
- Webhook URL later:
  - `https://your-subdomain.ngrok-free.dev/webhooks/github`

The repo-service should keep redirecting the browser back to the frontend after setup through:

- `FRONTEND_GITHUB_SETUP_REDIRECT_URL`

## Frontend

The frontend now talks to the gateway at:

- `http://localhost:8060`
