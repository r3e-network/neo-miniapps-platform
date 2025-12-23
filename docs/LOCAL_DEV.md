# Local Development Setup

Complete guide for setting up the local k3s development environment for the Neo Service Layer platform.

## Overview

The local development stack provides a complete Kubernetes environment running on your machine with:

- **k3s**: Lightweight Kubernetes distribution
- **MarbleRun Coordinator**: TEE orchestration (simulation mode)
- **Traefik**: Ingress controller with TLS
- **cert-manager**: Automatic TLS certificate management
- **Self-signed certificates**: For `*.localhost` domains

## Prerequisites

### System Requirements

- **OS**: Linux (Ubuntu 20.04+ recommended) or macOS
- **CPU**: 4+ cores (8+ recommended)
- **Memory**: 8GB+ RAM (16GB+ recommended)
- **Disk**: 20GB+ free space
- **Architecture**: x86_64 (ARM64 experimental)

### Required Tools

```bash
# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Install Docker (for building images)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Install Go 1.21+ (for building services)
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/bin/go/bin' >> ~/.bashrc
```

## Quick Start

### 1. Bootstrap k3s Local Stack

Run the automated setup script:

```bash
# Install complete k3s dev stack
./scripts/k3s-local-setup.sh install

# Or use Make target
make dev-stack-up
```

This will:

1. Install k3s with Traefik ingress
2. Create all required namespaces
3. Install cert-manager
4. Set up self-signed TLS certificates
5. Deploy MarbleRun coordinator (simulation mode)

### 2. Verify Installation

```bash
# Check status
./scripts/k3s-local-setup.sh status

# Or use Make target
make dev-stack-status

# View all pods
kubectl get pods -A
```

Expected output:

```
✓ k3s: Running
✓ cert-manager: Installed
✓ MarbleRun: Installed
✓ Traefik: Running
```

### 3. Deploy Services

```bash
# Build and deploy all services
make docker-build
make docker-up

# Or deploy to k3s
./scripts/deploy_k8s.sh --env dev
```

## Architecture

### Namespaces

The local stack creates the following namespaces:

- `marblerun`: MarbleRun coordinator
- `service-layer`: Core services (neoaccounts, neofeeds, etc.)
- `supabase`: Database and auth services (DEVSTACK-2)
- `platform`: Edge gateway and admin console (DEVSTACK-3/4)
- `cert-manager`: Certificate management
- `kube-system`: k3s system components

### Network Layout

```
┌─────────────────────────────────────────────────────────────┐
│                    Traefik Ingress                          │
│              (*.localhost with self-signed TLS)             │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────▼────────┐  ┌─────────▼────────┐  ┌────────▼────────┐
│   MarbleRun    │  │  Service Layer   │  │    Supabase     │
│  Coordinator   │  │    Services      │  │   (future)      │
│                │  │                  │  │                 │
│ :4433 (client) │  │ neoaccounts:8085 │  │ postgres:5432   │
│ :2001 (mesh)   │  │ neofeeds:8083    │  │ postgrest:3000  │
└────────────────┘  │ neoflow:8084     │  └─────────────────┘
                    │ ...              │
                    └──────────────────┘
```

### Service URLs (Local)

All services are accessible via `*.localhost` domains:

- **MarbleRun Coordinator**: `https://coordinator.marblerun.localhost:4433`
- **Services**: Port-forward individual services as needed
- **Supabase** (future): `https://supabase.localhost`
- **Admin Console** (future): `https://admin.localhost`

## Common Operations

### Start/Stop Dev Stack

```bash
# Start everything
make dev-stack-up

# Stop everything
make dev-stack-down

# Check status
make dev-stack-status
```

### Access MarbleRun Coordinator

```bash
# Port forward to coordinator
kubectl -n marblerun port-forward svc/coordinator-client-api 4433:4433

# In another terminal, set manifest
marblerun manifest set manifests/manifest.json localhost:4433 --insecure

# Check status
marblerun status localhost:4433 --insecure
```

### Access Services

```bash
# Port forward to a service
kubectl -n service-layer port-forward svc/neoaccounts 8085:8085

# Test the service
curl http://localhost:8085/health
```

### View Logs

```bash
# All pods in namespace
kubectl -n service-layer logs -f --all-containers=true -l app=neoaccounts

# Specific pod
kubectl -n service-layer logs -f <pod-name>

# MarbleRun coordinator
kubectl -n marblerun logs -f deployment/coordinator
```

### Rebuild and Redeploy

```bash
# Rebuild images
make docker-build

# Import to k3s
for service in neoaccounts neofeeds neoflow; do
  docker save service-layer/$service:dev | sudo k3s ctr images import -
done

# Restart deployments
kubectl -n service-layer rollout restart deployment
```

## Troubleshooting

### k3s Won't Start

```bash
# Check k3s service
sudo systemctl status k3s

# View k3s logs
sudo journalctl -u k3s -f

# Restart k3s
sudo systemctl restart k3s
```

### Pods Stuck in Pending

```bash
# Check pod events
kubectl -n service-layer describe pod <pod-name>

# Check node resources
kubectl describe nodes

# Check PVC status
kubectl get pvc -A
```

### Certificate Issues

```bash
# Check certificate status
kubectl -n cert-manager get certificates

# Describe certificate
kubectl -n cert-manager describe certificate wildcard-localhost

# Check cert-manager logs
kubectl -n cert-manager logs -f deployment/cert-manager
```

### MarbleRun Coordinator Not Ready

```bash
# Check coordinator logs
kubectl -n marblerun logs -f deployment/coordinator

# Check coordinator status
kubectl -n marblerun get pods

# Verify simulation mode
kubectl -n marblerun get deployment coordinator -o yaml | grep OE_SIMULATION
# Should show: value: "1"
```

### Clean Slate Reset

```bash
# Complete cleanup
make dev-stack-down
./scripts/k3s-local-setup.sh cleanup

# Reinstall
./scripts/k3s-local-setup.sh install
```

## Configuration

### Environment Variables

```bash
# k3s version
export K3S_VERSION=v1.28.5+k3s1

# cert-manager version
export CERT_MANAGER_VERSION=v1.14.0

# Installation timeout
export INSTALL_TIMEOUT=300
```

### Customization

Edit the following files to customize your setup:

- `k8s/namespaces.yaml`: Add/modify namespaces
- `k8s/ingress/wildcard-cert.yaml`: Add more DNS names
- `k8s/marblerun/overlays/simulation/coordinator.yaml`: Adjust coordinator resources
- `scripts/k3s-local-setup.sh`: Modify installation steps

## Development Workflow

### Typical Development Cycle

1. **Start dev stack**

    ```bash
    make dev-stack-up
    ```

2. **Make code changes**

    ```bash
    # Edit Go code in services/
    vim services/neoaccounts/service.go
    ```

3. **Run tests**

    ```bash
    make test
    ```

4. **Rebuild and deploy**

    ```bash
    make docker-build
    ./scripts/deploy_k8s.sh --env dev
    ```

5. **Test changes**

    ```bash
    kubectl -n service-layer port-forward svc/neoaccounts 8085:8085
    curl http://localhost:8085/health
    ```

6. **View logs**
    ```bash
    kubectl -n service-layer logs -f deployment/neoaccounts
    ```

### Hot Reload (Future)

For faster iteration, consider using:

- **Skaffold**: Automatic rebuild and redeploy on file changes
- **Tilt**: Visual dev environment with live updates
- **Telepresence**: Local development with remote cluster

## Next Steps

After setting up the local dev stack:

1. **DEVSTACK-2**: Set up self-hosted Supabase
    - See `.claude/specs/local-dev-stack/dev-plan.md`

2. **DEVSTACK-3**: Deploy Edge gateway
    - Configure service mesh and routing

3. **DEVSTACK-4**: Set up Admin Console
    - Monitor services and test MiniApps

## Resources

- [k3s Documentation](https://docs.k3s.io/)
- [MarbleRun Documentation](https://docs.edgeless.systems/marblerun/)
- [cert-manager Documentation](https://cert-manager.io/docs/)
- [Traefik Documentation](https://doc.traefik.io/traefik/)

## Support

For issues or questions:

1. Check the troubleshooting section above
2. Review logs: `kubectl logs -n <namespace> <pod-name>`
3. Check cluster events: `kubectl get events -A --sort-by='.lastTimestamp'`
4. Consult the development plan: `.claude/specs/local-dev-stack/dev-plan.md`
