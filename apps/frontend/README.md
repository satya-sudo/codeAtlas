# Frontend

React + Parcel frontend shell for CodeAtlas.

## Run locally

```bash
cd apps/frontend
npm install
npm start
```

Default port:

- `6060`

## Current integration points

- starts GitHub login through `http://localhost:8061/auth/github/login`
- expects auth-service to redirect back to `/auth/callback?token=...`
- verifies logged-in state through `GET /auth/me`

## Next frontend hookups

- replace mock overview data with `repo-service` APIs
- wire repository onboarding flow
- connect graph view to Neo4j-backed graph API
