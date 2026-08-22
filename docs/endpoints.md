# API Reference

## Overview

The shiyao daemon exposes sandbox lifecycle and execution endpoints.

## Endpoints

- `POST /v1/sandboxes`
- `GET /v1/sandboxes/{id}`
- `DELETE /v1/sandboxes/{id}`
- `GET /v1/health`

## Cloud team access tokens

Team access tokens are a Shiyao Cloud multi-tenancy feature. Team members can
list a team's tokens; only team owners and administrators can create or revoke
them. Build the daemon with the `cloud` tag to expose these endpoints.

- `POST /v1/teams/{team_id}/tokens`
- `GET /v1/teams/{team_id}/tokens`
- `DELETE /v1/teams/{team_id}/tokens/{id}`
