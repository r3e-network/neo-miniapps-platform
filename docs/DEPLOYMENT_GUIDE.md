# Deployment Guide

## Overview

This guide covers deploying the Neo Service Layer in production environments with MarbleRun/EGo and MarbleRun.

## Prerequisites

### Hardware Requirements

- **CPU**: Intel Xeon with MarbleRun support (Ice Lake or newer recommended)
- **RAM**: Minimum 16GB, recommended 32GB+
- **Storage**: 100GB+ SSD
- **Network**: Stable internet connection with public IP

### Software Requirements

- **OS**: Ubuntu 20.04 LTS or 22.04 LTS
- **Kernel**: 5.11+ with MarbleRun driver support
- **Docker**: 20.10+
- **Kubernetes**: 1.24+ (for production clusters)
- **Go**: 1.24+ (for building from source)

### MarbleRun Setup

1. **Enable MarbleRun in BIOS**
   ```bash
   # Verify MarbleRun is enabled
   cpuid | grep MarbleRun
   ```

2. **Install MarbleRun Driver**
   ```bash
   # For DCAP driver (recommended)
   wget https://download.01.org/intel-sgx/latest/linux-latest/distro/ubuntu22.04-server/sgx_linux_x64_driver_2.11.0_2d2b795.bin
   chmod +x sgx_linux_x64_driver_2.11.0_2d2b795.bin
   sudo ./sgx_linux_x64_driver_2.11.0_2d2b795.bin
   ```

3. **Install MarbleRun PSW (Platform Software)**
   ```bash
   echo 'deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu jammy main' | sudo tee /etc/apt/sources.list.d/intel-sgx.list
   wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | sudo apt-key add -
   sudo apt update
   sudo apt install -y libsgx-enclave-common libsgx-dcap-ql sgx-aesm-service

   # Verify AESM is running (required by many SGX/DCAP setups)
   sudo systemctl status aesmd --no-pager
   ```

4. **Verify MarbleRun Installation**
   ```bash
   ls /dev/sgx_*
   # Should show: /dev/tee /dev/sgx_provision
   ```

---

## Deployment Options

### Option 1: Docker Compose (Development/Testing)

Best for: Local development, testing, small deployments

#### 1. Clone Repository

```bash
git clone https://github.com/R3E-Network/service_layer.git
cd service_layer
```

#### 2. Configure Environment

```bash
cp .env.example .env
nano .env
```

Note: `scripts/up.sh` (and Docker Compose) will load `./.env` by default. To use a
different env file, run `./scripts/up.sh --env-file PATH`. To ignore `./.env`,
run `./scripts/up.sh --no-env-file`.

Required environment variables:

```bash
# Runtime
MARBLE_ENV=development  # development|testing|production

# MarbleRun Coordinator
# Docker Compose: marbles reach the coordinator via service DNS (bridge network)
COORDINATOR_MESH_ADDR=coordinator:2001
COORDINATOR_CLIENT_ADDR=localhost:4433
OE_SIMULATION=1  # 1=simulation (dev), 0=SGX hardware

# Database
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_SERVICE_KEY=your-supabase-service-key

# Neo N3 Network
NEO_RPC_URL=https://mainnet1.neo.org:443
NEO_NETWORK_MAGIC=860833102

# Supabase Edge (Gateway)
# The user-facing gateway is Supabase Edge Functions and is configured in your
# Supabase project environment (not in this Docker/K8s stack).
#
# Minimum env vars for Edge Functions:
# SUPABASE_URL=https://your-project.supabase.co
# SUPABASE_ANON_KEY=your-anon-key
# SUPABASE_SERVICE_ROLE_KEY=your-service-role-key
# SECRETS_MASTER_KEY=<hex 32 bytes>
# NEOFEEDS_URL=http://neofeeds:8083
# NEOCOMPUTE_URL=http://neocompute:8086
# NEOVRF_URL=http://neovrf:8087
# TXPROXY_URL=http://txproxy:8090
# CONTRACT_PAYMENTHUB_HASH=0x...
# CONTRACT_GOVERNANCE_HASH=0x...
# CONTRACT_RANDOMNESSLOG_HASH=0x... (optional; for RNG anchoring)
```

#### 3. Start Services

```bash
# Simulation mode (OE_SIMULATION=1): no SGX hardware required (MarbleRun still runs in simulation)
make docker-up

# SGX hardware mode (OE_SIMULATION=0)
make docker-up-sgx
```

#### 4. Set / Re-apply MarbleRun Manifest (optional)

`make docker-up` / `make docker-up-sgx` already applies the manifest via `scripts/up.sh`. Run this only if you need to re-apply:

```bash
make marblerun-manifest
```

#### 5. Verify Deployment

```bash
# Check MarbleRun status
marblerun status localhost:4433 --insecure

# Example: check an internal service health endpoint from inside the mesh
docker compose -f docker/docker-compose.simulation.yaml exec neocompute \
  sh -c 'wget -qO- http://localhost:8086/health || curl -fsS http://localhost:8086/health'
```

---

### Option 2: Kubernetes (Production)

Best for: Production deployments, high availability, scalability

#### 1. Prepare Kubernetes Cluster

```bash
# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Verify cluster access
kubectl cluster-info
```

#### 2. Install MarbleRun Coordinator

```bash
# Install MarbleRun CLI
wget https://github.com/edgelesssys/marblerun/releases/latest/download/marblerun-linux-amd64
sudo install marblerun-linux-amd64 /usr/local/bin/marblerun

# Install MarbleRun on Kubernetes
marblerun install --domain service-layer.neo.org
```

#### 3. Create Namespace

```bash
kubectl create namespace service-layer
```

#### 4. Configure Secrets

```bash
# Create the runtime Secret used by the services (Supabase service key, OAuth client secrets, ...)
cp k8s/secrets.yaml.template k8s/secrets.yaml
# edit k8s/secrets.yaml
kubectl apply -f k8s/secrets.yaml
#
# Alternatively, if you manage secrets in `.env`, you can generate/apply the Secret directly:
#   ./scripts/apply_k8s_secrets_from_env.sh --namespace service-layer --name service-layer-secrets

# Create TLS certificates (if not using cert-manager)
kubectl create secret tls service-layer-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key \
  --namespace=service-layer
```

#### 5. Deploy Services

```bash
# Apply Kubernetes manifests via Kustomize overlays
kubectl apply -k k8s/overlays/simulation
# production settings (replicas/env); for real SGX hardware, prefer the SGX overlay below
kubectl apply -k k8s/overlays/production
# SGX hardware (adds SGX device-plugin limits and mounts AESM socket)
kubectl apply -k k8s/overlays/sgx-hardware
# Optional: hardened variants (read-only root filesystem + writable /tmp)
# Pick ONE of these instead of the corresponding non-hardened overlay above.
kubectl apply -k k8s/overlays/production-hardened
kubectl apply -k k8s/overlays/sgx-hardware-hardened

# Or use the deployment script
# - dev: deploys the simulation overlay (OE_SIMULATION=1)
# - test: deploys the test overlay (OE_SIMULATION=1, MARBLE_ENV=testing)
# - prod: deploys the production overlay (OE_SIMULATION=0) and requires signed images that match `manifests/manifest.json`
./scripts/deploy_k8s.sh --env dev
```

Production security notes:

- The production overlay applies NetworkPolicies for both ingress and egress.
  - If your ingress controller runs in a different namespace, update `k8s/overlays/production/networkpolicy.yaml`.
  - If your node CIDR(s) are not RFC1918 ranges, update the `allow-node-health-probes` rule in `k8s/overlays/production/networkpolicy.yaml`.
  - If you require outbound ports other than 443 (not recommended), update `k8s/overlays/production/networkpolicy-egress.yaml`.
  - External HTTPS egress (443) is allowed by default, but private/link-local/loopback ranges are excluded to reduce SSRF risk; adjust if you use PrivateLink/VPC endpoints.

#### 6. Set MarbleRun Manifest

```bash
# Get coordinator client API address (4433)
COORDINATOR_CLIENT_ADDR=$(kubectl get svc -n marblerun coordinator-client-api -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# Set manifest
marblerun manifest set manifests/manifest.json "$COORDINATOR_CLIENT_ADDR:4433"
```

#### 7. Verify Deployment

```bash
# Check pod status
kubectl get pods -n service-layer

# Check service endpoints
kubectl get svc -n service-layer

# Check logs
kubectl logs -n service-layer -l app=neofeeds --tail=100

# Verify attestation
marblerun manifest verify --coordinator-addr $COORDINATOR:4433
```

---

## Configuration

### MarbleRun Manifest

The manifest defines the trusted execution environment topology (enclave
workloads only). The user-facing gateway is Supabase Edge and is **not** part of
the MarbleRun manifest.

See: `manifests/manifest.json`.

**Signer IDs and build keys:** `SignerID` is derived from the enclave signing key (MRSIGNER). For production deployments you must use a stable signing key (stored securely in CI/build secrets) so rebuilt images continue to match the manifest. Never ship or commit the enclave signing private key (`private.pem`) in runtime images or source control.

#### Building signed images (SGX hardware / production)

The Docker images sign enclaves during build (`ego sign`). In SGX hardware mode this must use **stable signing keys** that match `Packages.*.SignerID`, otherwise the MarbleRun Coordinator will refuse to activate marbles.

Docker Compose cannot pass BuildKit secrets, so for SGX hardware you should build images via `docker build` (or CI) and start Compose with `--no-build`.

Example (local build with BuildKit secrets):

```bash
# Service (example: neocompute uses SERVICE=neocompute)
DOCKER_BUILDKIT=1 docker build \
  --secret id=ego_private_key,src=/path/to/neocompute-private.pem \
  --build-arg EGO_STRICT_SIGNING=1 \
  --build-arg SERVICE=neocompute \
  -f docker/Dockerfile.service \
  -t service-layer/neocompute:latest \
  .

# Start without building (uses the images you built/pulled)
docker compose -f docker/docker-compose.yaml up -d --no-build
```

Helper (recommended): `./scripts/up.sh` supports `--signing-key` / `--signing-key-dir` to build signed images and verify `SignerID`s against `manifests/manifest.json` before starting the stack.

### Service Configuration

Services are configured via environment variables (see `.env.example`, `config/*.env`, and `k8s/base/configmap.yaml`). Sensitive values should live in Kubernetes Secrets (`k8s/secrets.yaml.template`) or MarbleRun manifest secrets.

Example (Supabase Edge gateway env, configured in your Supabase project):

```bash
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_ANON_KEY=your-anon-key
SUPABASE_SERVICE_ROLE_KEY=your-service-role-key
SECRETS_MASTER_KEY=$(openssl rand -hex 32)
NEOFEEDS_URL=https://neofeeds:8083
NEOCOMPUTE_URL=https://neocompute:8086
NEOVRF_URL=https://neovrf:8087
TXPROXY_URL=https://txproxy:8090
CONTRACT_PAYMENTHUB_HASH=0x...
CONTRACT_GOVERNANCE_HASH=0x...
```

Example (oracle):

```bash
# Secrets are stored in Supabase; the master key is injected by MarbleRun in Compose/K8s.
# For non-Marblerun deployments:
# SECRETS_MASTER_KEY=$(openssl rand -hex 32)
# Required in production/SGX mode (non-empty, valid prefixes). Requests only allow `https://` targets in strict identity mode.
ORACLE_HTTP_ALLOWLIST=https://api.binance.com,https://api.coinbase.com
```

---

## Monitoring

### Prometheus Metrics

All services expose Prometheus metrics at `/metrics`.

Production notes:

- By default, this repo does **not** expose `/metrics` publicly. Scrape from inside the cluster (or via port-forward) instead.
- In MarbleRun SGX mode, service endpoints (including `/metrics`) are typically protected by MarbleRun mTLS, which means a standard Prometheus instance cannot scrape them unless it can present a valid client certificate (or you run Prometheus inside the mesh).

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'service-layer'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - service-layer
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: neofeeds|neoflow|neoaccounts|neocompute|neooracle|globalsigner|txproxy
```

### Grafana Dashboards

Create dashboards in Grafana using the exported Prometheus metrics.

### Logging

Logs are structured JSON and can be collected with Fluentd/Fluent Bit:

```yaml
# fluent-bit.conf
[INPUT]
    Name              tail
    Path              /var/log/containers/*service-layer*.log
    Parser            docker
    Tag               kube.*

[OUTPUT]
    Name              es
    Match             kube.*
    Host              elasticsearch
    Port              9200
    Index             service-layer
```

---

## Backup and Recovery

### Database Backup

```bash
# Backup Supabase database
pg_dump -h your-db-host -U postgres -d service_layer > backup.sql

# Restore
psql -h your-db-host -U postgres -d service_layer < backup.sql
```

### Secrets Backup

```bash
# Export Kubernetes runtime Secret (contains Supabase service key, JWT secret, etc.)
kubectl get secret service-layer-secrets -n service-layer -o yaml > service-layer-secrets-backup.yaml

# Backup MarbleRun manifest
marblerun manifest get "$COORDINATOR_CLIENT_ADDR:4433" > manifest-backup.json
```

### MarbleRun Coordinator PVC Backup (Seal State)

The Coordinator stores its sealed state under `/coordinator/data` on a RWO PVC (`coordinator-pvc`) in the `marblerun` namespace.
Back up this PVC before upgrades and during regular DR drills.

```bash
# Create a local backup tarball (+ .sha256) under ./backups/
./scripts/coordinator_backup.sh

# Optional: upload to S3 (requires aws CLI + credentials)
./scripts/coordinator_backup.sh --s3-uri s3://<bucket>/<prefix>/
```

### MarbleRun Coordinator PVC Restore

Restoring the PVC is a destructive operation for the existing seal dir contents. The restore script will:
scale down `deployment/coordinator`, mount the PVC via a helper pod, restore the archive, then scale the Deployment back up.

```bash
# Restore from a local backup
./scripts/coordinator_restore.sh ./backups/coordinator-pvc-<timestamp>.tar.gz

# Or restore directly from S3
./scripts/coordinator_restore.sh --s3-uri s3://<bucket>/<prefix>/coordinator-pvc-<timestamp>.tar.gz

# Verify coordinator comes back
kubectl rollout status deployment/coordinator -n marblerun --timeout=10m
```

### Disaster Recovery

1. **Restore Kubernetes cluster**
2. **Reinstall MarbleRun coordinator**
3. **Restore Coordinator PVC (seal state)**: `./scripts/coordinator_restore.sh <backup.tar.gz>`
4. **Restore Secret**: `kubectl apply -f service-layer-secrets-backup.yaml`
5. **Restore manifest**: `marblerun manifest set manifest-backup.json "$COORDINATOR_CLIENT_ADDR:4433"`
6. **Redeploy services**: `kubectl apply -k k8s/overlays/production` (or `k8s/overlays/simulation`)

---

## Security Hardening

### Network Security

```bash
# Configure firewall
sudo ufw allow 4433/tcp  # MarbleRun coordinator client API (only if you need remote access)
sudo ufw enable

# Use network policies in Kubernetes
# Provide your own NetworkPolicies (not shipped by default)
# kubectl apply -f network-policies.yaml
```

### Secret Management

```bash
# Use sealed secrets for GitOps workflows
# kubeseal --format yaml < k8s/secrets.yaml > sealed-secrets.yaml

# Or use external secret managers
# - HashiCorp Vault
# - AWS Secrets Manager
# - Azure Key Vault
```

### Regular Updates

```bash
# Update MarbleRun platform software
sudo apt update && sudo apt upgrade libsgx-*

# Update Docker images
docker pull r3enetwork/service-layer:latest

# Update Kubernetes deployments
kubectl set image deployment/neofeeds neofeeds=service-layer/neofeeds:latest -n service-layer
```

---

## Scaling

### Horizontal Scaling

```bash
# Scale a service (example: neofeeds)
kubectl scale deployment neofeeds --replicas=5 -n service-layer

# Auto-scaling based on CPU
kubectl autoscale deployment neofeeds \
  --cpu-percent=70 \
  --min=3 \
  --max=10 \
  -n service-layer
```

### Load Balancing

Public traffic is intended to go to **Supabase Edge** (gateway). Internal TEE
services are kept cluster-internal by default and should be accessed via
port-forward or a mesh/in-cluster client.

---

## Troubleshooting

### Common Issues

#### 1. MarbleRun Device Not Found

```bash
# Check MarbleRun driver
ls /dev/sgx_*

# Reinstall driver if missing
sudo apt install --reinstall sgx-driver
```

#### 2. MarbleRun Connection Failed

```bash
# Check coordinator status
kubectl get pods -n marblerun

# Check coordinator logs
kubectl logs -n marblerun -l app=coordinator

# Verify network connectivity
telnet coordinator-mesh-api.marblerun 2001
```

#### 3. Service Won't Start

```bash
# Check pod logs
kubectl logs -n service-layer <pod-name>

# Check events
kubectl describe pod -n service-layer <pod-name>

# Verify services
kubectl get pods -n service-layer
```

#### 4. Attestation Verification Failed

```bash
# Verify manifest
marblerun manifest verify --coordinator-addr $COORDINATOR:4433

# Check MarbleRun quote
marblerun certificate chain --coordinator-addr $COORDINATOR:4433

# Verify time synchronization
timedatectl status
```

### Debug Mode

Enable debug logging:

```bash
# Set environment variable
export LOG_LEVEL=debug

# Or in Kubernetes
kubectl set env deployment/neofeeds LOG_LEVEL=debug -n service-layer
```

---

## Performance Tuning

### Database Optimization

```sql
-- Create indexes for frequently queried fields
CREATE INDEX idx_accounts_locked_by ON accounts(locked_by);

-- Analyze query performance
EXPLAIN ANALYZE SELECT * FROM accounts WHERE locked_by = 'neocompute';
```

### Connection Pooling

```yaml
database:
  max_open_conns: 100
  max_idle_conns: 10
  conn_max_lifetime: 1h
```

### Caching

```yaml
cache:
  enabled: true
  type: redis
  redis_url: redis://redis:6379
  ttl: 300s
```

---

## Maintenance

### Regular Tasks

- **Daily**: Check logs and metrics
- **Weekly**: Review security alerts, update dependencies
- **Monthly**: Backup verification, disaster recovery drill
- **Quarterly**: Security audit, performance review

### Update Procedure

1. **Test in staging environment**
2. **Create backup**
3. **Update one service at a time**
4. **Verify functionality**
5. **Monitor for issues**
6. **Rollback if necessary**

```bash
# Rolling update
kubectl set image deployment/neofeeds neofeeds=service-layer/neofeeds:v1.1.0 -n service-layer
kubectl rollout status deployment/neofeeds -n service-layer

# Rollback if needed
kubectl rollout undo deployment/neofeeds -n service-layer
```

---

## Support

For deployment assistance:

- **Documentation**: https://docs.service-layer.neo.org
- **GitHub Issues**: https://github.com/R3E-Network/service_layer/issues
- **Discord**: https://discord.gg/neo
- **Email**: devops@r3e-network.org
