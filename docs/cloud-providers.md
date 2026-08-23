# Cloud providers guide

Yok is cloud-agnostic by design: every infrastructure touchpoint goes through a small
provider seam. This guide documents each seam, how to add a provider, and the operational
chores around authentication bootstrap.

## Adapter points

| Concern | Seam | AWS today | Azure target |
|---|---|---|---|
| Build compute | `CLOUD_PROVIDER` (api, TypeScript) | ECS Fargate tasks | Container Instances Jobs / Container Apps Jobs |
| Artifact storage | `STORAGE_PROVIDER` (build-server, JavaScript) | S3 | Blob Storage |
| Log transport | kafkajs client (api consumer, build-server producer) | Kafka | Event Hubs Kafka endpoint |
| Log store | ClickHouse client (api) | ClickHouse | Self-hosted ClickHouse or managed equivalent |
| Postgres | Prisma (`DATABASE_URL`) | any PostgreSQL | Azure Database for PostgreSQL |
| Artifact origin serving | `ARTIFACT_BASE_URL` + `OriginResolver` (reverse proxy, Go) | public S3 URLs | Blob via CDN or SAS URLs |

Notes on the Azure targets:

- **Compute**: Container Instances Jobs (or Container Apps Jobs) replace the Fargate
  `RunTaskCommand`. The build container receives `PROJECT_ID`, `DEPLOYMENT_ID`,
  `GIT_REPO_URL` and `FRAMEWORK` as environment variables; keep passing those.
- **Storage**: a Blob Storage adapter using `@azure/storage-blob`. Object keys stay
  `<OUTPUT_PREFIX>/<deploymentId>/<file>` with default prefix `__output`, so the reverse
  proxy keeps resolving `<ARTIFACT_BASE_URL>/<deploymentId>/...` unchanged.
- **Log transport**: Event Hubs exposes a Kafka-compatible endpoint. Set
  `KAFKA_BROKER=<namespace>.servicebus.windows.net:9093` and use SASL PLAIN where the
  username is the literal string `$ConnectionString` and the password is your full
  Event Hubs connection string. kafkajs needs no code change. If TLS CA validation needs a
  custom CA, `KAFKA_CA_PATH` is already supported on both producer and consumer.
- **Artifact serving**: private Blob containers have no public URL per object like an
  S3 bucket policy allows. Serve artifacts via CDN in front of the container, or via
  SAS URLs. The latter likely requires a signed-URL variant of the reverse proxy's
  `OriginResolver` (fetch/derive a short-lived URL per request instead of appending a path).

## Adding a provider

Each seam follows the same recipe: implement the interface, register a case in the factory,
select it via env.

### Compute provider (TypeScript, api)

1. Implement `ComputeProvider` from `api/src/providers/types.ts`. The interface is one
   method: `runBuildTask(input): Promise<void>` where `input` carries `projectId`,
   `deploymentId`, `gitRepoUrl` and `framework`.
2. Add e.g. `api/src/providers/azure/ci.ts`, mirroring `aws/ecs.ts`: read provider-specific
   variables through a typed `configFromEnv()` helper that fails fast on missing values.
3. Extend the `z.enum([...])` for `CLOUD_PROVIDER` in `api/src/config.ts`.
4. Add a case for the new value in `api/src/providers/factory.ts`.
5. Set `CLOUD_PROVIDER=azure` plus your provider-specific variables in the environment.

### Storage adapter (JavaScript, build-server)

1. Implement the adapter shape from `build-server/src/storage/types.js`:
   `putFile(key, body, contentType)`.
2. Add e.g. `build-server/src/storage/azure-blob.js` using `@azure/storage-blob`, reading its
   own credentials from env in the constructor.
3. Register a case (`azure`) in `build-server/src/storage/factory.js`.
4. Set `STORAGE_PROVIDER=azure`.

### Origin resolver (Go, reverse proxy)

1. Implement the `OriginResolver` interface from `reverse-proxy/origin.go`:
   `Resolve(deploymentID string) (*url.URL, error)`.
2. For plain URL templates no Go code is needed: point `ARTIFACT_BASE_URL` at your CDN or
   container base URL and the existing S3 resolver serves any host.
3. For signed URLs, add a constructor next to `NewS3OriginResolver` in `reverse-proxy/origin.go`
   and select it from an env var in `main.go`.
4. Keep returning proxies through `proxyFor` so one `httputil.ReverseProxy` stays cached per
   origin.

## Operations

### Cancelling deployments

`POST /deployment/:id/cancel` (used by `yok cancel`) flips the deployment status in the
database only. It does **not** cancel the running build task (e.g., the ECS Fargate task),
because task ARNs are not persisted; the compute task keeps running until it finishes.

### Claiming legacy projects

Projects created before PAT auth have `user_id = NULL` and are invisible to ownership-scoped
queries. Claim them for your admin user with SQL against the application database:

```sql
UPDATE "Project" SET "user_id" = '<admin-user-id>' WHERE "user_id" IS NULL;
```

### First-run bootstrap sequence

1. Deploy the API stack (`docker compose up -d`).
2. Create the admin user while zero users exist:

   ```bash
   curl -X POST https://<api-host>/auth/bootstrap \
     -H 'Content-Type: application/json' \
     -d '{"email":"you@example.com"}'
   ```

   Once users exist this endpoint requires the `X-Bootstrap-Secret` header matching
   `BOOTSTRAP_SECRET`.
3. Save the returned raw token immediately (it is shown once). This is your admin token;
   run `yok login` and paste it.
4. Create the proxy service account: call `POST /auth/bootstrap` again with a dedicated
   email and the `X-Bootstrap-Secret` header. Save the returned raw token.
5. Set that token as `PROXY_SERVICE_TOKEN` in your root `.env` file.
6. Restart the stack so api and reverse-proxy pick it up:
   `docker compose up -d --force-recreate`.

## Prisma generator note

The Prisma schema intentionally retains the `prisma-client-js` generator. Migrating to the
newer `prisma-client` generator is a tracked follow-up, deliberately out of scope for the
current refactor.
