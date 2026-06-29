# `github`

Shared GitHub helpers for CodeAtlas.

Current responsibilities:

- build GitHub App installation URLs from the configured slug
- load the GitHub App private key PEM
- generate a GitHub App JWT
- exchange an installation ID for an installation access token

The frontend and public HTTP handlers should never receive installation access tokens directly. Those tokens are intended for backend-only calls to GitHub APIs.
