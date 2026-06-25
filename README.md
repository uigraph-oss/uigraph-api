# uigraph-api

[![license](https://img.shields.io/badge/license-BUSL--1.1-blue)](LICENSE)

Go REST API for [UiGraph](https://github.com/uigraph-oss) — the backend that powers authentication, organizations, architecture diagrams, service catalogs, maps, and document management.

## Features

- **Authentication** — sessions, OAuth/OIDC SSO, SAML, and service accounts
- **Organizations** — members, teams, roles (admin / editor / viewer), and invitations
- **Diagrams** — versioned React Flow diagrams with thumbnail generation
- **Maps** — system maps with frames, focal points, and canvas sync
- **Service catalog** — services, API groups, endpoints, and database schemas
- **Docs** — global and per-service documentation with file assets
- **Object storage** — MinIO, S3, or GCS for uploads and presigned URLs
- **Vector search** — Qdrant or S3 Vectors with Ollama, Bedrock, or OpenAI embeddings
- **MCP usage tracking** — token and request metrics for AI integrations

## Architecture

```
cmd/api/                    entry point
internal/
  api/                      HTTP handlers and router
  store/postgres/           Postgres persistence
  middleware/               auth (session, bearer, API key)
  authz/                    RBAC and scoped service accounts
  migrate/                  embedded SQL migrations
  bootstrap/                first-run org + admin seed
migrations/                 numbered SQL migration files
```

On startup the server connects to Postgres, applies pending migrations, bootstraps a default org and admin user when the database is empty, then serves HTTP.

## Local development

The fastest way to run the full stack locally is through [uigraph-deploy](../uigraph-deploy):

```bash
cd ../uigraph-deploy
make docker-up
```

The API listens on `http://localhost:8080`. Health checks: `GET /healthz`, `GET /livez`.

To run the API binary directly (requires Postgres, Redis, and object storage):

```bash
go run ./cmd/api
```

Minimum environment variables:

| Variable | Description |
|---|---|
| `POSTGRES_URL` | Postgres connection string |
| `UIGRAPH_SECRET_KEY` | AES-256-GCM key — generate with `openssl rand -hex 32` |
| `REDIS_URL` | Redis connection string (caching and job queue) |
| `STORAGE_ENDPOINT` | Object storage endpoint (e.g. MinIO or S3) |
| `STORAGE_ACCESS_KEY` | Object storage access key |
| `STORAGE_SECRET_KEY` | Object storage secret key |

See [uigraph-deploy `.env.example`](../uigraph-deploy/.env.example) for the full configuration reference.

## Testing

```bash
# Unit tests (no database required)
go test ./internal/... -count=1

# Integration tests (requires Postgres)
TEST_POSTGRES_URL=postgres://uigraph:devpassword@localhost:5432/uigraph?sslmode=disable \
  go test ./tests/... -v -count=1
```

## License

This project is licensed under the [Business Source License 1.1](LICENSE) (BUSL-1.1).

- **Source available today** — you can read, modify, and redistribute the code under the terms of the license.
- **Non-production use** — free for development, testing, evaluation, and internal proof-of-concept.
- **Production use** — requires a commercial license from UiGraph. Production use means any use that supports the ongoing operation of your business or organization.
- **Future open source** — each version automatically converts to [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0) four years after it is first published under BUSL.

BUSL is not an OSI-approved open source license during the initial term. For commercial licensing questions, open an issue or contact the maintainers.

## Related projects

- [uigraph-ui](https://github.com/uigraph-oss/uigraph-ui) — web application
- [uigraph-graphql](https://github.com/uigraph-oss/uigraph-graphql) — GraphQL BFF
- [uigraph-gateway](https://github.com/uigraph-oss/uigraph-gateway) — CLI sync API
- [uigraph-mcp](https://github.com/uigraph-oss/uigraph-mcp) — MCP server for AI assistants
- [uigraph-sdk](https://github.com/uigraph-oss/uigraph-sdk) — TypeScript SDK
- [uigraph-deploy](https://github.com/uigraph-oss/uigraph-deploy) — self-hosted deployment

