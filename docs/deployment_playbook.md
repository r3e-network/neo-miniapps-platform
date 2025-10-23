# Deployment Playbook

This playbook captures the operational steps for running the Neo N3 service
layer in development, staging, and production. It combines configuration,
infrastructure, and operational best practices derived from the code base and
existing devops scripts.

## 1. Prerequisites

- Go 1.21+ (for local builds)
- Docker + Docker Compose (optional but recommended for parity)
- PostgreSQL 14+ (production/staging)
- Access to TLS certificates (production)
- Secrets store (e.g., Kubernetes secrets, AWS Secrets Manager, Vault) for API
  tokens and encryption keys

## 2. Configuration Overview

Environment variables (or CLI flags) drive most settings:

| Variable / Flag           | Description                                    | Default            |
|---------------------------|------------------------------------------------|--------------------|
| `DATABASE_URL` / `-dsn`   | PostgreSQL DSN; omit for in-memory mode        | In-memory store    |
| `API_TOKENS` / `-api-tokens` | Comma separated bearer tokens             | Required for prod  |
| `SECRET_ENCRYPTION_KEY`   | 16/24/32 byte key (raw/base64/hex) for AES-GCM | mandatory with DB  |
| `PRICEFEED_FETCH_URL`     | External HTTP price service                    | (optional)         |
| `PRICEFEED_FETCH_KEY`     | Bearer token for price fetcher                 | (optional)         |
| `ORACLE_RESOLVER_URL` / `_KEY` | Oracle callback resolver endpoint     | (optional)         |
| `GASBANK_RESOLVER_URL` / `_KEY` | Gas bank withdrawal resolver endpoint | (optional)         |
| `LOG_LEVEL` (`-log-level`) | Overrides default `info` level               | info               |

Sample configuration file: `configs/examples/appserver.json`.

## 3. Deployment Modes

### 3.1 Local Development

1. Start in-memory runtime:

   ```bash
   go run ./cmd/appserver -api-tokens dev-token
   ```

2. Access the API at `http://localhost:8080`. Use bearer token `dev-token`.

3. Seed data via examples or `curl` using endpoints from
   `docs/api_reference.md`.

### 3.2 Docker Compose

1. Copy environment template:

   ```bash
   cp .env.example .env
   ```

   Set:
   ```
   DATABASE_URL=postgres://service_layer:service_layer@db:5432/service_layer?sslmode=disable
   API_TOKENS=prod-token
   SECRET_ENCRYPTION_KEY=<32-byte-base64>
   ```

2. Launch services:

   ```bash
   docker compose up --build -d
   ```

3. Verify health:

   ```bash
   curl http://localhost:8080/healthz
   curl -H "Authorization: Bearer prod-token" http://localhost:8080/accounts
   ```

4. Logs:

   ```bash
   docker compose logs -f app
   ```

### 3.3 Kubernetes (Reference)

1. Create secrets:

   ```bash
   kubectl create secret generic service-layer-secrets \
     --from-literal=API_TOKENS=prod-token \
     --from-literal=SECRET_ENCRYPTION_KEY=<hex-or-base64-key> \
     --from-literal=DATABASE_URL=postgres://...
   ```

2. Deploy PostgreSQL (use managed service or available Helm chart).

3. Apply deployment (`k8s/appserver-deployment.yaml` example snippet):

   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: service-layer
   spec:
     replicas: 2
     selector:
       matchLabels:
         app: service-layer
     template:
       metadata:
         labels:
           app: service-layer
       spec:
         containers:
           - name: app
             image: ghcr.io/r3e-network/service-layer:<tag>
             ports:
               - containerPort: 8080
             envFrom:
               - secretRef:
                   name: service-layer-secrets
             readinessProbe:
               httpGet:
                 path: /healthz
                 port: 8080
             livenessProbe:
               httpGet:
                 path: /healthz
                 port: 8080
             resources:
               requests:
                 cpu: "250m"
                 memory: "256Mi"
               limits:
                 cpu: "1"
                 memory: "512Mi"
   ```

4. Expose via Service/Ingress (ensure TLS termination through ingress or a
   service mesh).

## 4. Database Migrations

`cmd/appserver` runs migrations automatically when connected to PostgreSQL. To
run migrations manually:

```bash
go run ./cmd/appserver -dsn "postgres://..." -migrate-only
```

## 5. TLS and Networking

- Terminate TLS at ingress/load balancer or enable Go TLS flags.
- Restrict inbound traffic to trusted networks; enforce API tokens.
- If running price/oracle/gasbank resolvers, allow outbound HTTPS to those
  services and restrict via firewall rules.

## 6. Secrets Management

- Do **not** store `SECRET_ENCRYPTION_KEY` or API tokens in plain config maps.
- Prefer cloud KMS or Vault; inject at runtime via environment variables or
  mounted files.
- Rotate secrets regularly; the secrets service automatically increments version
  numbers on update.

## 7. Observability

- **Metrics:** scrape `/metrics` with Prometheus. Key metrics include function
  execution latency (`function_execution_duration_seconds`) and automation
  scheduler counters.
- **Logging:** structured text via Logrus. Direct logs to stdout and aggregate
  with ELK/Datadog. Adjust log level using `LOG_LEVEL` env.
- **Health:** monitor `/healthz` (readiness) and optionally add synthetic tests
  hitting `/accounts`.

## 8. Scaling & High Availability

- Stateless app; scale horizontally.
- Use PostgreSQL HA (e.g., managed service, Patroni).
- Enable job schedulers (automation, price refresher, gasbank settlement, oracle
  dispatcher) on multiple replicas but ensure idempotency. Use leader election
  if duplication becomes an issue.
- Configure resource requests/limits to avoid CPU starvation for the Goja TEE.

## 9. Backup & Disaster Recovery

- Database: schedule automated backups (pg_dump, managed service snapshots).
- Secrets: rely on database backups; encryption keys must be backed up in the
  secrets store.
- Config: version control for manifests and `.env` templates (without secrets).

## 10. Release Process

1. Run unit/integration tests: `go test ./...`.
2. Build container image: `docker build -t ghcr.io/r3e-network/service-layer:<tag> .`
3. Push image and update deployment manifests.
4. Apply rollout (`kubectl rollout restart deployment/service-layer`).
5. Monitor health/metrics after deployment.

## 11. Troubleshooting

| Symptom | Checks / Fixes |
|---------|----------------|
| 401 Unauthorized | Ensure `Authorization` header matches configured token |
| `secret resolver not configured` | Confirm secrets service wired and encryption key present |
| Automation jobs running too often | Verify `next_run` in automation store, ensure scheduler interval configured |
| Gas bank withdrawal stuck | Check resolver endpoint availability, inspect `/metrics` and logs |
| Oracle requests never complete | Validate resolver credentials and status transitions via API |

## 12. Reference Links

- [API Reference](docs/api_reference.md)
- [Service Layer Guide](docs/service_layer_guide.md)
- [Runtime Quick Start](docs/runtime_quickstart.md)
- [DevOps Setup](docs/devops_setup.md)

Keep this playbook versioned with the code base. Update it on every release that
changes configuration, infrastructure, or operational behaviour.

