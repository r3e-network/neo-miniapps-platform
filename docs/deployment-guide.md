# Production Deployment Guide

Complete guide for deploying the Service Layer to production environments.

## Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         PRODUCTION ARCHITECTURE                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌──────────────┐     ┌──────────────┐     ┌──────────────┐           │
│   │   Load       │────▶│   Service    │────▶│  PostgreSQL  │           │
│   │   Balancer   │     │   Layer API  │     │   (Primary)  │           │
│   └──────────────┘     └──────────────┘     └──────────────┘           │
│         │                     │                     │                   │
│         │              ┌──────┴──────┐      ┌──────┴──────┐            │
│         │              │             │      │             │            │
│         ▼              ▼             ▼      ▼             ▼            │
│   ┌──────────────┐   ┌─────┐   ┌─────┐   ┌─────┐   ┌──────────┐       │
│   │   Dashboard  │   │ Neo │   │ Neo │   │ PG  │   │ RocketMQ │       │
│   │   (Static)   │   │ RPC │   │ RPC │   │Rep. │   │ (Events) │       │
│   └──────────────┘   │Main │   │Test │   └─────┘   └──────────┘       │
│                      └─────┘   └─────┘                                 │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Prerequisites

### Hardware Requirements

| Component | Minimum | Recommended | Notes |
|-----------|---------|-------------|-------|
| **API Server** | 2 CPU, 4GB RAM | 4 CPU, 8GB RAM | Scale horizontally |
| **PostgreSQL** | 2 CPU, 8GB RAM | 4 CPU, 16GB RAM | SSD required |
| **Dashboard** | 1 CPU, 512MB RAM | 2 CPU, 1GB RAM | Static files |

### Software Requirements

- Docker 24+ / Kubernetes 1.28+
- PostgreSQL 14+ (15+ recommended)
- TLS certificates (Let's Encrypt or commercial)
- DNS records configured

---

## Environment Configuration

### Required Variables

```bash
# Database (required for production)
DATABASE_URL=postgres://user:password@host:5432/service_layer?sslmode=require

# Authentication (CHANGE THESE!)
API_TOKENS=<random-secure-token-1>,<random-secure-token-2>
AUTH_USERS=admin:<bcrypt-hashed-password>:admin
AUTH_JWT_SECRET=<random-64-char-secret>

# Encryption (required)
SECRET_ENCRYPTION_KEY=<32-byte-hex-or-base64-key>

# Server
LISTEN_ADDR=:8080
```

### Optional Variables

```bash
# Randomness
RANDOM_SIGNING_KEY=<ed25519-private-key-base64>

# Oracle integration
ORACLE_TTL_SECONDS=300
ORACLE_MAX_ATTEMPTS=5
ORACLE_DLQ_ENABLED=true
ORACLE_RUNNER_TOKENS=<runner-token-1>,<runner-token-2>

# GasBank settlement
GASBANK_RESOLVER_URL=https://settlement.example.com
GASBANK_RESOLVER_KEY=<api-key>
GASBANK_POLL_INTERVAL=15s
GASBANK_MAX_ATTEMPTS=5

# Price feed
PRICEFEED_FETCH_URL=https://prices.example.com
PRICEFEED_FETCH_KEY=<api-key>

# Data feed aggregation
DATAFEEDS_AGGREGATION=median

# NEO integration
NEO_RPC_URL=https://neo.example.com:10332
NEO_SNAPSHOT_DIR=/data/snapshots
NEO_STABLE_BUFFER=12

# Observability
MODULE_SLOW_MS=2000
AUDIT_LOG_PATH=/var/log/service-layer-audit.jsonl

# RocketMQ (optional event bus)
ROCKETMQ_NAMESERVER=mq.example.com:9876
ROCKETMQ_TOPIC_PREFIX=sl-prod
ROCKETMQ_CONSUMER_GROUP=sl-prod-consumers
```

---

## Docker Deployment

### Production Compose

```yaml
# docker-compose.prod.yml
version: '3.8'

services:
  appserver:
    image: ghcr.io/r3e-network/service_layer:latest
    restart: always
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: ${DATABASE_URL}
      API_TOKENS: ${API_TOKENS}
      AUTH_JWT_SECRET: ${AUTH_JWT_SECRET}
      SECRET_ENCRYPTION_KEY: ${SECRET_ENCRYPTION_KEY}
      LISTEN_ADDR: ":8080"
    volumes:
      - ./configs/config.yaml:/app/configs/config.yaml:ro
      - ./logs:/var/log/service-layer
      - ./snapshots:/app/snapshots
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/readyz"]
      interval: 10s
      timeout: 5s
      retries: 3
    depends_on:
      postgres:
        condition: service_healthy
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '2'
          memory: 4G
        reservations:
          cpus: '1'
          memory: 2G

  postgres:
    image: postgres:15-alpine
    restart: always
    environment:
      POSTGRES_USER: serviceuser
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: service_layer
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U serviceuser -d service_layer"]
      interval: 5s
      timeout: 5s
      retries: 5
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G

  dashboard:
    image: ghcr.io/r3e-network/service_layer-dashboard:latest
    restart: always
    ports:
      - "8081:80"
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 256M

volumes:
  postgres_data:
```

### Deploy

```bash
# Create environment file
cp .env.example .env.prod
# Edit .env.prod with production values

# Deploy
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d

# Check status
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs -f appserver
```

---

## Kubernetes Deployment

### Namespace and Secrets

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: service-layer
---
# secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: service-layer-secrets
  namespace: service-layer
type: Opaque
stringData:
  DATABASE_URL: "postgres://user:pass@postgres.service-layer:5432/service_layer?sslmode=require"
  API_TOKENS: "your-secure-token"
  AUTH_JWT_SECRET: "your-jwt-secret"
  SECRET_ENCRYPTION_KEY: "your-32-byte-key"
```

### Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: service-layer
  namespace: service-layer
spec:
  replicas: 3
  selector:
    matchLabels:
      app: service-layer
  template:
    metadata:
      labels:
        app: service-layer
    spec:
      containers:
      - name: appserver
        image: ghcr.io/r3e-network/service_layer:latest
        ports:
        - containerPort: 8080
        envFrom:
        - secretRef:
            name: service-layer-secrets
        resources:
          requests:
            memory: "2Gi"
            cpu: "1"
          limits:
            memory: "4Gi"
            cpu: "2"
        livenessProbe:
          httpGet:
            path: /livez
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        volumeMounts:
        - name: config
          mountPath: /app/configs
          readOnly: true
      volumes:
      - name: config
        configMap:
          name: service-layer-config
---
apiVersion: v1
kind: Service
metadata:
  name: service-layer
  namespace: service-layer
spec:
  selector:
    app: service-layer
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
```

### Ingress with TLS

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: service-layer
  namespace: service-layer
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - api.service-layer.example.com
    secretName: service-layer-tls
  rules:
  - host: api.service-layer.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: service-layer
            port:
              number: 80
```

### Deploy to Kubernetes

```bash
kubectl apply -f namespace.yaml
kubectl apply -f secrets.yaml
kubectl apply -f configmap.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f ingress.yaml

# Check status
kubectl -n service-layer get pods
kubectl -n service-layer logs -f deployment/service-layer
```

---

## Database Setup

### Initial Setup

```bash
# Create database
psql -U postgres
CREATE USER serviceuser WITH PASSWORD 'your-password';
CREATE DATABASE service_layer OWNER serviceuser;
GRANT ALL PRIVILEGES ON DATABASE service_layer TO serviceuser;
\q

# Run migrations (automatic on startup with -migrate flag)
./bin/appserver -dsn "$DATABASE_URL" -migrate
```

### Backup Strategy

```bash
# Daily backup script
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
pg_dump -h postgres-host -U serviceuser service_layer | gzip > /backups/sl_$DATE.sql.gz

# Retain 30 days
find /backups -name "sl_*.sql.gz" -mtime +30 -delete
```

### Connection Pooling

For high-traffic deployments, use PgBouncer:

```ini
# pgbouncer.ini
[databases]
service_layer = host=postgres-host port=5432 dbname=service_layer

[pgbouncer]
listen_port = 6432
listen_addr = *
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 20
```

---

## Security Hardening

### TLS Configuration

```bash
# Generate strong DH params
openssl dhparam -out /etc/ssl/dhparam.pem 2048
```

Nginx config:
```nginx
server {
    listen 443 ssl http2;
    server_name api.service-layer.example.com;

    ssl_certificate /etc/letsencrypt/live/api.service-layer.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.service-layer.example.com/privkey.pem;
    ssl_dhparam /etc/ssl/dhparam.pem;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_prefer_server_ciphers off;

    add_header Strict-Transport-Security "max-age=31536000" always;
    add_header X-Content-Type-Options nosniff;
    add_header X-Frame-Options DENY;

    location / {
        proxy_pass http://appserver:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Rate Limiting

```nginx
# Rate limiting zone
limit_req_zone $binary_remote_addr zone=api:10m rate=100r/s;

server {
    location /accounts {
        limit_req zone=api burst=50 nodelay;
        proxy_pass http://appserver:8080;
    }
}
```

### Network Security

```yaml
# Kubernetes NetworkPolicy
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: service-layer-network
  namespace: service-layer
spec:
  podSelector:
    matchLabels:
      app: service-layer
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: postgres
    ports:
    - port: 5432
```

### Secret Management

For production, use a secrets manager:

```yaml
# Using External Secrets Operator with AWS Secrets Manager
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: service-layer-secrets
  namespace: service-layer
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: ClusterSecretStore
  target:
    name: service-layer-secrets
  data:
  - secretKey: DATABASE_URL
    remoteRef:
      key: service-layer/prod/database-url
  - secretKey: API_TOKENS
    remoteRef:
      key: service-layer/prod/api-tokens
  - secretKey: SECRET_ENCRYPTION_KEY
    remoteRef:
      key: service-layer/prod/encryption-key
```

---

## Monitoring

### Prometheus Metrics

The Service Layer exposes `/metrics` in Prometheus format:

```yaml
# prometheus.yaml
scrape_configs:
  - job_name: 'service-layer'
    static_configs:
      - targets: ['service-layer:8080']
    metrics_path: /metrics
    scheme: http
```

### Key Metrics

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `http_requests_total` | Total HTTP requests | Rate > 10k/min |
| `http_request_duration_seconds` | Request latency | p99 > 1s |
| `service_module_status` | Module health | != "started" |
| `db_connections_active` | Active DB connections | > 80% pool |

### Alerting Rules

```yaml
# alerts.yaml
groups:
- name: service-layer
  rules:
  - alert: ServiceLayerDown
    expr: up{job="service-layer"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Service Layer is down"

  - alert: HighLatency
    expr: histogram_quantile(0.99, http_request_duration_seconds_bucket{job="service-layer"}) > 1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High API latency"

  - alert: ModuleFailed
    expr: service_module_status{status="failed"} > 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Service module {{ $labels.module }} failed"
```

### Grafana Dashboard

Import the included dashboard from `docs/grafana/service-layer-dashboard.json`.

---

## Logging

### Structured Logging

```bash
# JSON format for log aggregation
export LOG_FORMAT=json
export LOG_LEVEL=info
```

### Log Aggregation

```yaml
# Fluent Bit config for Kubernetes
[INPUT]
    Name              tail
    Path              /var/log/containers/service-layer*.log
    Parser            docker
    Tag               service-layer.*

[FILTER]
    Name              kubernetes
    Match             service-layer.*
    Merge_Log         On

[OUTPUT]
    Name              elasticsearch
    Match             *
    Host              elasticsearch.logging
    Port              9200
    Index             service-layer
```

### Audit Logging

```bash
# Enable audit logging
export AUDIT_LOG_PATH=/var/log/service-layer-audit.jsonl

# Query audits via API
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://localhost:8080/admin/audit?limit=100&method=POST"
```

---

## High Availability

### Multi-Region Setup

```
┌─────────────────────────────────────────────────────────────────┐
│                        GLOBAL LOAD BALANCER                      │
│                     (Cloudflare / AWS Global)                    │
└───────────────────────────┬─────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│   US-EAST     │   │   EU-WEST     │   │   AP-SOUTH    │
├───────────────┤   ├───────────────┤   ├───────────────┤
│ Service Layer │   │ Service Layer │   │ Service Layer │
│ (3 replicas)  │   │ (3 replicas)  │   │ (3 replicas)  │
├───────────────┤   ├───────────────┤   ├───────────────┤
│   PostgreSQL  │◄─►│   PostgreSQL  │◄─►│   PostgreSQL  │
│   (Primary)   │   │   (Replica)   │   │   (Replica)   │
└───────────────┘   └───────────────┘   └───────────────┘
```

### Database Replication

```sql
-- On primary
CREATE PUBLICATION service_layer_pub FOR ALL TABLES;

-- On replica
CREATE SUBSCRIPTION service_layer_sub
CONNECTION 'host=primary-host port=5432 dbname=service_layer'
PUBLICATION service_layer_pub;
```

---

## Troubleshooting

### Common Issues

**503 Service Unavailable**
```bash
# Check pod status
kubectl -n service-layer get pods
kubectl -n service-layer describe pod <pod-name>

# Check logs
kubectl -n service-layer logs -f deployment/service-layer
```

**Database Connection Errors**
```bash
# Test connectivity
psql "$DATABASE_URL" -c "SELECT 1"

# Check connection pool
curl http://localhost:8080/system/status | jq '.modules[] | select(.name == "store-postgres")'
```

**High Memory Usage**
```bash
# Check Go memory stats
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

### Health Checks

```bash
# Liveness (is the process running?)
curl http://localhost:8080/livez

# Readiness (is it ready to serve traffic?)
curl http://localhost:8080/readyz

# Detailed status
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/system/status | jq
```

---

## Maintenance

### Rolling Updates

```bash
# Kubernetes
kubectl -n service-layer set image deployment/service-layer \
  appserver=ghcr.io/r3e-network/service_layer:new-version

# Docker Compose
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d --no-deps appserver
```

### Database Migrations

```bash
# Migrations run automatically on startup
# For manual control:
./bin/appserver -dsn "$DATABASE_URL" -migrate-only
```

### Backup Verification

```bash
# Weekly restore test
pg_restore -d service_layer_test /backups/latest.sql.gz
./bin/appserver -dsn "postgres://...service_layer_test" -migrate
curl http://localhost:8080/readyz
```

---

## Checklist

### Pre-Deployment

- [ ] TLS certificates configured
- [ ] Strong passwords generated
- [ ] Secrets in secure storage
- [ ] Database backups configured
- [ ] Monitoring/alerting set up
- [ ] Network policies applied
- [ ] Rate limiting configured

### Post-Deployment

- [ ] Health checks passing
- [ ] All modules started
- [ ] Metrics being collected
- [ ] Logs being aggregated
- [ ] Alerts configured
- [ ] Documentation updated

---

## Related Documentation

- [Security Hardening](security-hardening.md)
- [Operations Runbook](ops-runbook.md)
- [Architecture Layers](architecture-layers.md)
