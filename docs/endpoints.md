# API Reference

## Overview

The shiyao daemon exposes sandbox lifecycle and execution endpoints.

## Endpoints

- `POST /v1/sandboxes`
- `GET /v1/sandboxes/{id}`
- `DELETE /v1/sandboxes/{id}`
- `GET /v1/health`

## Team access tokens

Team members can list a team's tokens. Only team owners and administrators can
create or revoke them.

- `POST /v1/teams/{team_id}/tokens`
- `GET /v1/teams/{team_id}/tokens`
- `DELETE /v1/teams/{team_id}/tokens/{id}`
