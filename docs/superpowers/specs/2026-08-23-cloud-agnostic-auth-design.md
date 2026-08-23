# Yok: Cloud-Agnostic Refactor + PAT Auth — Design Spec

Date: 2026-08-23
Status: Approved
Scope: api/, build-server/, reverse-proxy/, cli/

## Context

Yok is a self-hosted PaaS (API + build server + reverse proxy + CLI). All infrastructure
coupling is AWS-specific today, and the API has zero authentication. This spec covers:

1. Provider seams so Azure becomes a drop-in later (no Azure code shipped yet)
2. Personal access token (PAT) authentication
3. Explicit deployment status events replacing log-string inference
4. Code cleanups discovered during analysis

Decisions made with the owner:

- **Seams only** — extract interfaces + keep working AWS implementations; no untested Azure code.
- **PAT auth now** — GitHub/Stripe-style tokens, hashed at rest, CLI `login/logout`.
- **Keep Kafka + ClickHouse**, wrapped behind small modules. Note: Azure Event Hubs speaks the Kafka protocol, so kafkajs likely survives the migration unchanged.

## Analysis findings this design addresses

| # | Finding | Fix |
|---|---------|-----|
| F1 | `api/index.ts` is a 650-line monolith | Modular restructure (§1) |
| F2 | No auth anywhere; `GET /project` lists all users' projects; deploys trigger paid ECS builds for anyone | PAT auth + ownership scoping (§2) |
| F3 | Deployment status inferred by string-matching logs (`includes('Starting')`, `"Build output uploaded to S3 successfully"`) duplicated in API and CLI | Structured status events on Kafka (§3) |
| F4 | Bug: build server reads `AWS_BUCKET_NAME`, compose provides `AWS_S3_BUCKET` | Neutral config names inside adapters (§4) |
| F5 | `child_process`, `fs`, `path` npm shims shadow Node built-ins in build-server deps | Remove from package.json (§4) |
| F6 | Hardcoded `yok.ninja` / `http://api.yok.ninja` across API responses, CLI, workflows | Config-driven URLs (§6) |
| F7 | Reverse proxy constructs a new `httputil.ReverseProxy` per request | Cache per-origin proxies (§5) |
| F8 | `exec("cd dir && cmd")` with async callbacks can drop output lines | `spawn(cmd, {cwd})` promise wrapper (§4) |

## §1 API restructure

```
api/src/
  index.ts               # bootstrap only: load config, assemble app, listen
  config.ts              # zod-typed env parsing
  db/prisma.ts           # PrismaClient singleton (plain Postgres)
  middleware/auth.ts     # PAT bearer middleware
  routes/
    projects.ts          # /project, /project/check, /project/:id, /project/:id/deployments
    deployments.ts       # /deploy, /deployment/:id, /deployment/:id/cancel, /logs/:id
    auth.ts              # /auth/tokens, /auth/tokens/:id, /auth/bootstrap, /auth/me
    resolve.ts           # /resolve/:slug (proxy-facing, service-token protected)
    health.ts            # /health (public)
  services/
    projectService.ts
    deploymentService.ts
    tokenService.ts      # generation/hashing/validation
  providers/
    types.ts             # interface ComputeProvider { runBuildTask(input): Promise<void> }
    factory.ts           # CLOUD_PROVIDER=aws → aws impl; azure added later as sibling dir
    aws/ecs.ts           # current Fargate RunTaskCommand code extracted unchanged
  bus/kafka.ts           # consumer wiring
  logs/clickhouse.ts     # log_events insert/query
```

- The API never touches object storage directly today, so exactly one provider interface is
  defined (`ComputeProvider`). Do not add speculative interfaces.
- Drop `@prisma/extension-accelerate`: it couples the app to Prisma's hosted platform. Plain
  Postgres via `DATABASE_URL` works identically on any cloud.
- Response shapes and route paths are unchanged (CLI compatibility).

### ComputeProvider interface

```ts
export interface BuildTaskInput {
  projectId: string;
  deploymentId: string;
  gitRepoUrl: string;
  framework: string;
}

export interface ComputeProvider {
  runBuildTask(input: BuildTaskInput): Promise<void>;
}
```

`aws/ecs.ts` implements this with the existing environment-variable contract
(`AWS_ECS_CLUSTER`, `AWS_ECS_TASK_DEFINITION`, `AWS_ECS_CONTAINER_NAME`,
`AWS_ECS_SUBNETS`, `AWS_ECS_SECURITY_GROUPS`, plus credentials/region).

Known limitation (documented, not fixed): `/deployment/:id/cancel` flips DB status only;
it does not cancel the underlying ECS task because task ARNs are not persisted.

## §2 PAT authentication

### Data model (Prisma migration)

```prisma
model User {
  id        String   @id @default(uuid())
  email     String   @unique
  createdAt DateTime @default(now()) @map("created_at")
  tokens    Token[]
  projects  Project[]
}

model Token {
  id         String    @id @default(uuid())
  user       User      @relation(fields: [userId], references: [id])
  userId     String    @map("user_id")
  name       String
  prefix     String    @unique            // first 12 chars incl. "yok_"
  hash       String    @unique            // sha256 hex of full raw token
  lastUsedAt DateTime? @map("last_used_at")
  expiresAt  DateTime  @map("expires_at") // default 90 days from creation
  revokedAt  DateTime? @map("revoked_at")
  createdAt  DateTime  @default(now()) @map("created_at")
}
```

`Project.userId` is added **nullable** (legacy rows become unowned; invisible to scoped
queries until claimed by bootstrap owner).

### Token lifecycle (per industry practice)

- Format: `yok_` + `crypto.randomBytes(32).toString('base64url')` (~43 chars).
- Store SHA-256 hex of the raw token (fast hash is correct here: tokens are high-entropy,
  unlike passwords). Raw value returned exactly once at creation.
- Validation: split prefix → indexed lookup by `prefix` → `timingSafeEqual(sha256(raw), hash)`
  → check `expiresAt > now && revokedAt == null`. `lastUsedAt` update is fire-and-forget.
- Default expiry 90 days.

### Endpoints

| Route | Auth | Behavior |
|---|---|---|
| `POST /auth/bootstrap` | public if zero users exist, else requires `BOOTSTRAP_SECRET` header | creates admin user, returns raw token once |
| `POST /auth/tokens` | bearer | create named token for self |
| `DELETE /auth/tokens/:id` | bearer | revoke own token |
| `GET /auth/me` | bearer | identity echo (used by `yok login`) |

### Protection rules

- Public: `/health`, `/auth/bootstrap`.
- Service-authenticated: `/resolve/:slug` — accepts a second PAT issued to a proxy "service
  user"; its raw token ships to the proxy as `PROXY_SERVICE_TOKEN`.
- Everything else: bearer middleware; requests resolve to `req.user`.

### Ownership scoping

- `GET /project` returns only projects owned by `req.user` (fixes F2).
- Project creation sets `userId`; deploy/logs/cancel/status verify ownership.
- Legacy unowned rows (`userId = null`) are invisible to scoped queries. Claiming them is an
  operator action outside this codebase (a SQL update setting `user_id`), documented in
  `docs/cloud-providers.md`'s operations section. No admin endpoint is built for this now.

## §3 Explicit status events

Kafka messages on the existing topic gain a discriminator:

```jsonc
{"type":"log","projectId":"…","deploymentId":"…","log":"…"}          // stored in ClickHouse
{"type":"status","projectId":"…","deploymentId":"…","status":"IN_PROGRESS"} // updates Deployment.status
```

- Status values whitelist-checked against the Prisma enum before writing.
- The API consumer routes by `type`; unknown types are logged and skipped.
- `updateDeploymentStatus()` string-matching is deleted.
- The CLI log streamer drops its duplicated S3-marker checks; completion/failure comes from
  status polling of `/deployment/:id`, which already exists.

## §4 Build server

Restructure (CommonJS JS, no compile step):

```
build-server/src/
  index.js          # orchestration: clone handled by main.sh → build → verify → upload
  bus/kafka.js      # producer; publishLog(), publishStatus()
  storage/
    types.js        # JSDoc-typed interface: putFiles(distDir, keyPrefix) => Promise<void>
    aws-s3.js       # current PutObjectCommand loop extracted
    factory.js      # STORAGE_PROVIDER=aws (azure later)
```

Cleanups bundled:

- Bucket read from `AWS_S3_BUCKET` only (fixes F4); adapter owns provider-prefixed vars.
- Remove `child_process`, `fs`, `path` npm shims (F5).
- `spawn(cmd, { cwd })` wrapped in a proper promise; stdout/stderr forwarded line-by-line to
  `publishLog` without async-callback races (F8).
- TLS CA path configurable via optional `KAFKA_CA_PATH` (unset = default TLS).
- Publishes `status` events: `IN_PROGRESS` at start, `COMPLETED` after upload, `FAILED` in catch.
- `main.sh` keeps doing the git clone; it passes repo URL through unchanged.

## §5 Reverse proxy

- Extract Go interface:

```go
type OriginResolver interface {
    Resolve(deploymentID string) (*url.URL, error)
}
```

- `s3OriginResolver` builds artifact base URL from `ARTIFACT_BASE_URL`
  (e.g. `https://mybucket.s3.eu-west-1.amazonaws.com/__output`) — replaces hand-concatenated
  S3 URLs. A future Azure resolver or signed-URL resolver implements the same interface.
- Cache one `httputil.ReverseProxy` per origin (sync.Map) instead of per-request construction (F7).
- Subdomain/slug logic already host-derived; unchanged.

## §6 CLI

- Config file (existing location) gains: `apiUrl` (default `https://api.yok.ninja`),
  `token`, `siteDomain` (default `yok.ninja`). File written with `0600`.
- Env overrides for CI: `YOK_TOKEN`, `YOK_API_URL`.
- New commands: `yok login` (prompt/paste token → validate against `/auth/me` → save),
  `yok logout` (clear token).
- Every API request attaches `Authorization: Bearer <token>`; all hardcoded
  `yok.ninja`/API URL literals replaced with config lookups (F6).
- Log streaming simplification per §3.

## §7 Configuration schema & docs

Neutral top-level variables per service; provider-specific credentials stay inside their
adapter modules:

```bash
CLOUD_PROVIDER=aws          # api: compute adapter selection
STORAGE_PROVIDER=aws        # build-server: storage adapter selection
ARTIFACT_BASE_URL=…         # reverse-proxy: artifact origin template
PROXY_SERVICE_TOKEN=…       # reverse-proxy + api
BOOTSTRAP_SECRET=…          # api
```

Deliverables:

- `.env.example` for api, build-server, reverse-proxy.
- `docs/cloud-providers.md`: adapter authoring guide + Azure mapping table:

| Concern | AWS today | Azure target |
|---|---|---|
| Build compute | ECS Fargate tasks | Container Instances Jobs / Container Apps Jobs |
| Artifact storage | S3 (+ public URL serving) | Blob Storage (+ CDN/SAS) |
| Log transport | Kafka (kafkajs) | Event Hubs Kafka endpoint (protocol-compatible) |
| Log store | ClickHouse | Self-hosted ClickHouse or managed equivalent |
| Postgres | any | Azure Database for PostgreSQL (Prisma-compatible) |

- README updated to reflect structure, auth flow, and configuration.

## §8 Testing & verification

New (minimal, high-value):

- vitest in `api/` covering `tokenService` round-trip properties (generate → validate ok,
  wrong token rejected, expired rejected, revoked rejected) and `config.ts` env parsing
  failure cases.
- No other test scaffolding (explicit scope decision).

Existing verification gates:

- `pnpm tsc --noEmit` (api), `go vet ./... && go build ./...` (cli, reverse-proxy),
  `docker compose config`.
- Documented manual smoke test: bootstrap → login → create project → deploy → follow logs →
  status transitions COMPLETED → cancel path returns error on terminal state.

## Non-goals

- Writing Azure adapter code (seams make it trivial later).
- OAuth/browser flows, keychain integration, refresh tokens (PAT pattern chosen deliberately).
- Splitting the Kafka consumer into a separate worker process (operational nicety, not needed
  at current scale).
- Actually cancelling running ECS tasks from `/cancel` (needs task ARN persistence; documented
  limitation).
