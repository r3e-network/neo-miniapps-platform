# Neo Service Layer Deployment Guide

Production deployment guide for MarbleRun + EGo + Supabase + Netlify architecture.

---

## Prerequisites

### Hardware Requirements

**Production (SGX)**:
- Intel CPU with SGX support (Xeon E3 v5+, Xeon Scalable)
- SGX enabled in BIOS
- 16GB+ RAM
- 100GB+ SSD

**Development (Simulation)**:
- Any modern x86_64 CPU
- 8GB+ RAM
- 50GB+ SSD

### Software Requirements

```bash
# Go 1.21+
go version

# EGo SDK (for SGX)
ego version

# Docker & Docker Compose
docker --version
docker-compose --version

# Kubernetes (production)
kubectl version

# Netlify CLI (frontend)
netlify --version

# Neo N3 tools
neo-express --version
```

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    Netlify (Frontend)                           │
│  • React + Vite + TailwindCSS                                   │
│  • Supabase Auth integration                                    │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                    Supabase Cloud/Self-hosted                   │
│  • PostgreSQL + PostgREST                                       │
│  • GoTrue (Auth)                                                │
│  • Realtime (WebSocket)                                         │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│              Kubernetes Cluster (SGX-enabled)                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              MarbleRun Coordinator                       │   │
│  └─────────────────────────────────────────────────────────┘   │
│  ┌───────────┬───────────┬───────────┬───────────┬─────────┐   │
│  │  Oracle   │    VRF    │  Secrets  │  GasBank  │  ...    │   │
│  │  Marble   │  Marble   │  Marble   │  Marble   │         │   │
│  └───────────┴───────────┴───────────┴───────────┴─────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 1. Development Deployment

### 1.1 Start Supabase (Local)

```bash
# Start Supabase services
docker-compose up -d supabase

# Verify services
curl http://localhost:54321/rest/v1/
curl http://localhost:54321/auth/v1/health
```

### 1.2 Run Coordinator (Simulation)

```bash
# Set environment
export SIMULATION_MODE=true
export COORDINATOR_ADDR=0.0.0.0:4433
export SUPABASE_URL=http://localhost:54321
export SUPABASE_SERVICE_KEY=your-service-key

# Run coordinator
go run ./cmd/coordinator
```

### 1.3 Run Service Marbles (Simulation)

```bash
# Terminal 1: Oracle service
SIMULATION_MODE=true MARBLE_TYPE=oracle COORDINATOR_ADDR=localhost:4433 go run ./cmd/marble

# Terminal 2: VRF service
SIMULATION_MODE=true MARBLE_TYPE=vrf COORDINATOR_ADDR=localhost:4433 go run ./cmd/marble

# Terminal 3: Secrets service
SIMULATION_MODE=true MARBLE_TYPE=secrets COORDINATOR_ADDR=localhost:4433 go run ./cmd/marble
```

### 1.4 Run Frontend

```bash
cd frontend

# Install dependencies
npm install

# Set environment
cp .env.example .env
# Edit .env with your Supabase credentials

# Start development server
npm run dev
```

---

## 2. Production Deployment

### 2.1 Supabase Setup

#### Option A: Supabase Cloud

1. Create project at https://supabase.com
2. Note your project URL and keys
3. Run migrations:

```bash
# Install Supabase CLI
npm install -g supabase

# Link to project
supabase link --project-ref your-project-ref

# Run migrations
supabase db push
```

#### Option B: Self-hosted Supabase

```bash
# Clone Supabase
git clone https://github.com/supabase/supabase
cd supabase/docker

# Configure
cp .env.example .env
# Edit .env with your settings

# Start
docker-compose up -d
```

### 2.2 Database Migrations

```sql
-- supabase/migrations/0001_core_schema.sql

-- Service requests table
CREATE TABLE service_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES auth.users(id),
    service_type TEXT NOT NULL,
    operation TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    priority INTEGER DEFAULT 0,
    result JSONB,
    error_message TEXT,
    tee_signature TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- RLS policies
ALTER TABLE service_requests ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own requests"
    ON service_requests FOR SELECT
    USING (auth.uid() = user_id);

CREATE POLICY "Users can insert own requests"
    ON service_requests FOR INSERT
    WITH CHECK (auth.uid() = user_id);

-- Realtime notifications
CREATE TABLE realtime_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES auth.users(id),
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    message TEXT,
    data JSONB,
    read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE realtime_notifications ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own notifications"
    ON realtime_notifications FOR SELECT
    USING (auth.uid() = user_id);

-- Enable realtime
ALTER PUBLICATION supabase_realtime ADD TABLE service_requests;
ALTER PUBLICATION supabase_realtime ADD TABLE realtime_notifications;
```

### 2.3 Build EGo Enclaves

```bash
# Install EGo
wget https://github.com/edgelesssys/ego/releases/download/v1.5.0/ego_1.5.0_amd64.deb
sudo dpkg -i ego_1.5.0_amd64.deb

# Build coordinator
cd coordinator
ego-go build -o coordinator .
ego sign coordinator

# Build marble
cd ../cmd/marble
ego-go build -o marble .
ego sign marble
```

### 2.4 MarbleRun Manifest

Create `manifest.json`:

```json
{
    "Packages": {
        "coordinator": {
            "UniqueID": "coordinator-unique-id",
            "SignerID": "your-signer-id",
            "ProductID": 1,
            "SecurityVersion": 1
        },
        "oracle": {
            "UniqueID": "oracle-unique-id",
            "SignerID": "your-signer-id",
            "ProductID": 2,
            "SecurityVersion": 1
        },
        "vrf": {
            "UniqueID": "vrf-unique-id",
            "SignerID": "your-signer-id",
            "ProductID": 3,
            "SecurityVersion": 1
        },
        "secrets": {
            "UniqueID": "secrets-unique-id",
            "SignerID": "your-signer-id",
            "ProductID": 4,
            "SecurityVersion": 1
        }
    },
    "Marbles": {
        "oracle": {
            "Package": "oracle",
            "Parameters": {
                "Env": {
                    "MARBLE_TYPE": "oracle",
                    "SUPABASE_URL": "{{ .Secrets.supabase_url }}"
                },
                "Files": {},
                "Argv": []
            }
        },
        "vrf": {
            "Package": "vrf",
            "Parameters": {
                "Env": {
                    "MARBLE_TYPE": "vrf",
                    "SUPABASE_URL": "{{ .Secrets.supabase_url }}"
                }
            }
        },
        "secrets": {
            "Package": "secrets",
            "Parameters": {
                "Env": {
                    "MARBLE_TYPE": "secrets",
                    "SUPABASE_URL": "{{ .Secrets.supabase_url }}"
                }
            }
        }
    },
    "Secrets": {
        "supabase_url": {
            "Type": "plain",
            "UserDefined": true
        },
        "supabase_service_key": {
            "Type": "plain",
            "UserDefined": true
        },
        "neo_rpc_url": {
            "Type": "plain",
            "UserDefined": true
        }
    },
    "Users": {
        "admin": {
            "Certificate": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"
        }
    }
}
```

### 2.5 Kubernetes Deployment

#### Install MarbleRun

```bash
# Install MarbleRun
helm repo add edgeless https://helm.edgeless.systems/stable
helm repo update

helm install marblerun edgeless/marblerun \
    --namespace marblerun \
    --create-namespace \
    --set coordinator.resources.limits.sgx.intel.com/epc=128Mi
```

#### Deploy Services

```yaml
# devops/kubernetes/coordinator.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: coordinator
  namespace: service-layer
spec:
  replicas: 1
  selector:
    matchLabels:
      app: coordinator
  template:
    metadata:
      labels:
        app: coordinator
    spec:
      containers:
      - name: coordinator
        image: your-registry/coordinator:latest
        ports:
        - containerPort: 4433
        resources:
          limits:
            sgx.intel.com/epc: 128Mi
        env:
        - name: COORDINATOR_ADDR
          value: "0.0.0.0:4433"
        - name: EDG_MARBLE_TYPE
          value: "coordinator"
---
apiVersion: v1
kind: Service
metadata:
  name: coordinator
  namespace: service-layer
spec:
  selector:
    app: coordinator
  ports:
  - port: 4433
    targetPort: 4433
```

```yaml
# devops/kubernetes/oracle-marble.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: oracle-marble
  namespace: service-layer
spec:
  replicas: 3
  selector:
    matchLabels:
      app: oracle-marble
  template:
    metadata:
      labels:
        app: oracle-marble
    spec:
      containers:
      - name: marble
        image: your-registry/marble:latest
        resources:
          limits:
            sgx.intel.com/epc: 64Mi
        env:
        - name: EDG_MARBLE_TYPE
          value: "oracle"
        - name: EDG_MARBLE_COORDINATOR_ADDR
          value: "coordinator:4433"
```

#### Apply Deployments

```bash
# Create namespace
kubectl create namespace service-layer

# Apply configurations
kubectl apply -f devops/kubernetes/

# Set manifest
marblerun manifest set manifest.json --coordinator coordinator.service-layer:4433

# Set secrets
marblerun secret set supabase_url https://your-project.supabase.co
marblerun secret set supabase_service_key eyJ...
marblerun secret set neo_rpc_url https://mainnet1.neo.coz.io:443
```

### 2.6 Frontend Deployment (Netlify)

#### Configure Netlify

Create `netlify.toml`:

```toml
[build]
  command = "npm run build"
  publish = "dist"

[build.environment]
  NODE_VERSION = "18"

[[redirects]]
  from = "/*"
  to = "/index.html"
  status = 200
```

#### Deploy

```bash
cd frontend

# Install Netlify CLI
npm install -g netlify-cli

# Login
netlify login

# Create site
netlify init

# Set environment variables
netlify env:set VITE_SUPABASE_URL https://your-project.supabase.co
netlify env:set VITE_SUPABASE_ANON_KEY eyJ...
netlify env:set VITE_NEO_RPC_URL https://mainnet1.neo.coz.io:443

# Deploy
netlify deploy --prod
```

---

## 3. Configuration Reference

### Environment Variables

| Variable | Component | Description |
|----------|-----------|-------------|
| `COORDINATOR_ADDR` | Coordinator | Listen address (default: 0.0.0.0:4433) |
| `SUPABASE_URL` | All | Supabase project URL |
| `SUPABASE_SERVICE_KEY` | Backend | Supabase service role key |
| `SIMULATION_MODE` | All | Enable SGX simulation (dev only) |
| `MARBLE_TYPE` | Marble | Service type (oracle, vrf, etc.) |
| `NEO_RPC_URL` | Backend | Neo N3 RPC endpoint |
| `VITE_SUPABASE_URL` | Frontend | Supabase URL for client |
| `VITE_SUPABASE_ANON_KEY` | Frontend | Supabase anon key |
| `VITE_NEO_RPC_URL` | Frontend | Neo RPC for client |

### Ports

| Port | Service | Protocol |
|------|---------|----------|
| 4433 | Coordinator | HTTPS/gRPC |
| 8080 | Service API | HTTP |
| 54321 | Supabase API | HTTP |
| 54322 | Supabase Studio | HTTP |
| 5432 | PostgreSQL | TCP |

---

## 4. Monitoring

### Health Checks

```bash
# Coordinator health
curl https://coordinator:4433/api/v2/status

# Marble health
curl http://marble:8080/health

# Supabase health
curl http://supabase:54321/rest/v1/
```

### Metrics

```bash
# Prometheus metrics
curl http://marble:8080/metrics
```

### Logs

```bash
# Kubernetes logs
kubectl logs -f deployment/oracle-marble -n service-layer

# Docker logs
docker-compose logs -f coordinator
```

---

## 5. Security Checklist

### Pre-deployment

- [ ] SGX enabled in BIOS
- [ ] DCAP attestation configured
- [ ] TLS certificates generated
- [ ] Secrets encrypted and stored securely
- [ ] RLS policies verified

### Post-deployment

- [ ] Attestation verification working
- [ ] mTLS between Marbles
- [ ] Supabase RLS enforced
- [ ] Rate limiting configured
- [ ] Monitoring alerts set up

---

## 6. Troubleshooting

### SGX Issues

```bash
# Check SGX support
cpuid | grep -i sgx

# Check EGo installation
ego env

# Run in simulation mode for testing
SIMULATION_MODE=true go run ./cmd/marble
```

### Coordinator Connection

```bash
# Test coordinator connectivity
curl -k https://coordinator:4433/api/v2/status

# Check marble activation logs
kubectl logs deployment/oracle-marble -n service-layer | grep activation
```

### Supabase Issues

```bash
# Test Supabase connection
curl http://localhost:54321/rest/v1/ \
  -H "apikey: your-anon-key"

# Check PostgREST logs
docker-compose logs postgrest
```

---

## 7. Scaling

### Horizontal Scaling

```bash
# Scale marbles
kubectl scale deployment oracle-marble --replicas=5 -n service-layer

# Scale with HPA
kubectl autoscale deployment oracle-marble \
  --min=3 --max=10 --cpu-percent=70 -n service-layer
```

### Database Scaling

- Use Supabase connection pooling (PgBouncer)
- Enable read replicas for heavy read workloads
- Consider partitioning for large tables

---

## Related Documentation

- [Architecture Overview](architecture/MARBLERUN_ARCHITECTURE.md)
- [Service Documentation](services/README.md)
- [MarbleRun Docs](https://docs.edgeless.systems/marblerun)
- [EGo Docs](https://docs.edgeless.systems/ego)
- [Supabase Docs](https://supabase.com/docs)
- [Netlify Docs](https://docs.netlify.com)
