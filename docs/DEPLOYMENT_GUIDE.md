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
   sudo apt install -y libsgx-enclave-common libsgx-dcap-ql
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

Required environment variables:

```bash
# MarbleRun Coordinator
MARBLE_COORDINATOR_ADDR=coordinator-mesh-api.marblerun:2001
MARBLE_TYPE=gateway  # or service name

# Database
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_KEY=your-supabase-anon-key
SUPABASE_SERVICE_KEY=your-supabase-service-key

# Neo N3 Network
NEO_RPC_URL=https://mainnet1.neo.org:443
NEO_NETWORK_MAGIC=860833102

# Security
JWT_SECRET=your-jwt-secret-min-32-chars
ENCRYPTION_KEY=your-encryption-key-32-bytes

# Services
ACCOUNTPOOL_URL=http://accountpool:8081
ORACLE_URL=http://neooracle:8082
VRF_URL=http://neorand:8083
```

#### 3. Start Services

```bash
# Simulation mode (no MarbleRun required)
make docker-up

# With MarbleRun hardware
make docker-up-tee
```

#### 4. Set MarbleRun Manifest

```bash
make marblerun-manifest
```

#### 5. Verify Deployment

```bash
# Check service health
curl http://localhost:8080/health

# Check MarbleRun status
marblerun status --coordinator-addr localhost:4433
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
# Create neostore from .env file
kubectl create secret generic service-layer-neostore \
  --from-env-file=.env \
  --namespace=service-layer

# Create TLS certificates (if not using cert-manager)
kubectl create secret tls service-layer-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key \
  --namespace=service-layer
```

#### 5. Deploy Services

```bash
# Apply Kubernetes manifests
kubectl apply -f k8s/ --namespace=service-layer

# Or use the deployment script
./scripts/deploy_k8s.sh
```

#### 6. Set MarbleRun Manifest

```bash
# Get coordinator address
COORDINATOR=$(kubectl get svc -n marblerun coordinator-mesh-api -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# Set manifest
marblerun manifest set manifests/manifest.json \
  --coordinator-addr $COORDINATOR:4433 \
  --era-config era-config.json
```

#### 7. Verify Deployment

```bash
# Check pod status
kubectl get pods -n service-layer

# Check service endpoints
kubectl get svc -n service-layer

# Check logs
kubectl logs -n service-layer -l app=gateway --tail=100

# Verify attestation
marblerun manifest verify --coordinator-addr $COORDINATOR:4433
```

---

## Configuration

### MarbleRun Manifest

The manifest defines the trusted execution environment topology:

```json
{
  "Packages": {
    "gateway": {
      "UniqueID": "gateway-unique-id",
      "SignerID": "gateway-signer-id",
      "ProductID": 1,
      "SecurityVersion": 1,
      "Debug": false
    }
  },
  "Marbles": {
    "gateway": {
      "Package": "gateway",
      "MaxActivations": 10,
      "Parameters": {
        "Env": {
          "MARBLE_TYPE": "gateway",
          "EDG_MARBLE_TYPE": "gateway"
        }
      }
    }
  },
  "Secrets": {
    "jwt_secret": {
      "Type": "symmetric-key",
      "Size": 32
    }
  }
}
```

### Service Configuration

Each service can be configured via environment variables or config files:

**Gateway (`config/gateway.yaml`):**

```yaml
server:
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

auth:
  jwt_secret: ${JWT_SECRET}
  token_expiry: 24h

rate_limit:
  requests_per_minute: 100
  burst: 200

services:
  neooracle: http://neooracle:8082
  neorand: http://neorand:8083
  neovault: http://neovault:8084
```

**Oracle (`config/neooracle.yaml`):**

```yaml
sources:
  - name: binance
    url: https://api.binance.com
    weight: 1.0
  - name: coinbase
    url: https://api.coinbase.com
    weight: 1.0

update_interval: 60s
price_deviation_threshold: 0.05
```

---

## Monitoring

### Prometheus Metrics

All services expose Prometheus metrics at `/metrics`:

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
        regex: service-layer-.*
```

### Grafana Dashboards

Import pre-built dashboards from `monitoring/grafana/`:

- `service-layer-overview.json` - Overall system health
- `service-layer-performance.json` - Performance metrics
- `service-layer-security.json` - Security and attestation metrics

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
# Export Kubernetes neostore
kubectl get neostore -n service-layer -o yaml > neostore-backup.yaml

# Backup MarbleRun manifest
marblerun manifest get --coordinator-addr $COORDINATOR:4433 > manifest-backup.json
```

### Disaster Recovery

1. **Restore Kubernetes cluster**
2. **Reinstall MarbleRun coordinator**
3. **Restore neostore**: `kubectl apply -f neostore-backup.yaml`
4. **Restore manifest**: `marblerun manifest set manifest-backup.json`
5. **Redeploy services**: `kubectl apply -f k8s/`

---

## Security Hardening

### Network Security

```bash
# Configure firewall
sudo ufw allow 8080/tcp  # Gateway
sudo ufw allow 4433/tcp  # MarbleRun coordinator
sudo ufw enable

# Use network policies in Kubernetes
kubectl apply -f k8s/network-policies.yaml
```

### Secret Management

```bash
# Use sealed neostore for GitOps
kubeseal --format yaml < neostore.yaml > sealed-neostore.yaml

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
kubectl set image deployment/gateway gateway=r3enetwork/service-layer:latest -n service-layer
```

---

## Scaling

### Horizontal Scaling

```bash
# Scale gateway replicas
kubectl scale deployment gateway --replicas=5 -n service-layer

# Auto-scaling based on CPU
kubectl autoscale deployment gateway \
  --cpu-percent=70 \
  --min=3 \
  --max=10 \
  -n service-layer
```

### Load Balancing

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: service-layer-ingress
  namespace: service-layer
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
    - hosts:
        - api.service-layer.neo.org
      secretName: service-layer-tls
  rules:
    - host: api.service-layer.neo.org
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: gateway
                port:
                  number: 8080
```

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

# Verify neostore
kubectl get neostore -n service-layer
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
kubectl set env deployment/gateway LOG_LEVEL=debug -n service-layer
```

---

## Performance Tuning

### Database Optimization

```sql
-- Create indexes for frequently queried fields
CREATE INDEX idx_accounts_locked_by ON accounts(locked_by);
CREATE INDEX idx_mix_requests_status ON mix_requests(status);

-- Analyze query performance
EXPLAIN ANALYZE SELECT * FROM accounts WHERE locked_by = 'neovault';
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
kubectl set image deployment/gateway gateway=r3enetwork/service-layer:v1.1.0 -n service-layer
kubectl rollout status deployment/gateway -n service-layer

# Rollback if needed
kubectl rollout undo deployment/gateway -n service-layer
```

---

## Support

For deployment assistance:

- **Documentation**: https://docs.service-layer.neo.org
- **GitHub Issues**: https://github.com/R3E-Network/service_layer/issues
- **Discord**: https://discord.gg/neo
- **Email**: devops@r3e-network.org
