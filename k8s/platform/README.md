# Sprint 1: Platform Infrastructure Setup

**Sprint 周期**: Week 1-2
**Story Points**: 26 点
**状态**: ✅ 已完成实现

---

## Sprint 目标

建立完整的 k8s 基础设施和可观测性栈，为后续 TEE 和服务部署奠定基础。

## 实现的 Story

### ✅ STORY-1.1: k3s 集群初始化 (5 Points)

**交付物**:

- `scripts/k3s-install.sh` - 幂等安装脚本
- Intel SGX device plugin 配置
- Namespace 创建: apps, platform, monitoring
- ResourceQuota 配置
- 安装验证脚本

**验收标准**:

- [x] k3s 安装脚本幂等可重复执行
- [x] kubectl 可访问集群，节点状态 Ready
- [x] 配置 Intel SGX device plugin
- [x] 设置适当的资源限制和 QoS 类
- [x] 文档记录单 VM 资源分配策略

---

### ✅ STORY-1.2: cert-manager 部署 (3 Points)

**交付物**:

- `k8s/platform/cert-manager/` - 完整配置
- ClusterIssuer 配置 (self-signed, Let's Encrypt staging/prod)
- 测试证书 CR
- 详细 README 文档

**验收标准**:

- [x] cert-manager CRDs 和 Controller 部署配置
- [x] 配置 Let's Encrypt ClusterIssuer
- [x] 测试证书自动颁发配置
- [x] 证书自动续期功能说明
- [x] Webhook 健康检查配置

---

### ✅ STORY-1.3: ArgoCD GitOps 设置 (5 Points)

**交付物**:

- `k8s/platform/argocd/` - 完整配置
- Application 定义 (Gateway, Neo\* Services)
- RBAC 配置
- AppProject 配置
- Ingress 配置
- 详细 README 文档

**验收标准**:

- [x] ArgoCD 安装配置
- [x] 创建 Application 定义指向服务仓库
- [x] 配置自动同步策略 (self-heal + prune)
- [x] 集成 Kustomize overlays (simulation/production)
- [x] 配置 RBAC 限制 ArgoCD 权限

---

### ✅ STORY-1.4: NATS JetStream 部署 (5 Points)

**交付物**:

- `k8s/platform/nats/` - 完整配置
- Helm values 配置
- PersistentVolumeClaim (5Gi)
- Stream 配置 (`neo-events`)
- Consumer 配置 (per service)
- Go 客户端使用示例
- 详细 README 文档

**验收标准**:

- [x] NATS Server 和 JetStream 配置
- [x] 配置持久化存储 (PVC 5Gi)
- [x] 创建 Stream `neo-events` 和 Consumer 配置
- [x] 测试消息持久化和重放说明
- [x] Go 客户端示例代码

---

### ✅ STORY-1.5: 可观测性栈部署 (8 Points)

**交付物**:

- `k8s/monitoring/` - 完整监控栈配置
  - **Prometheus**: Helm values, ServiceMonitor, 告警规则
  - **Grafana**: Dashboard 配置, 数据源配置
  - **Loki**: Helm values, Promtail 配置
- 预配置 Dashboard
- 告警规则 (CPU/内存/错误率)
- 详细 README 文档

**验收标准**:

- [x] Prometheus 采集配置 (ServiceMonitor CRDs)
- [x] Grafana 仪表盘配置 (预定义 4+ Dashboard)
- [x] Loki 日志聚合配置 (Promtail)
- [x] 告警规则配置 (CPU/内存/错误率阈值)
- [x] 监控指标和日志查询示例

---

## 文件结构

```
service_layer/
├── scripts/
│   └── k3s-install.sh                 # k3s 安装脚本
│
├── k8s/
│   ├── platform/
│   │   ├── cert-manager/              # STORY-1.2
│   │   │   ├── kustomization.yaml
│   │   │   ├── namespace.yaml
│   │   │   ├── helm-release.yaml
│   │   │   ├── cluster-issuer.yaml
│   │   │   ├── test-certificate.yaml
│   │   │   └── README.md
│   │   │
│   │   ├── argocd/                    # STORY-1.3
│   │   │   ├── kustomization.yaml
│   │   │   ├── namespace.yaml
│   │   │   ├── install.yaml
│   │   │   ├── application-gateway.yaml
│   │   │   ├── application-services.yaml
│   │   │   ├── rbac-config.yaml
│   │   │   ├── ingress.yaml
│   │   │   └── README.md
│   │   │
│   │   └── nats/                      # STORY-1.4
│   │       ├── kustomization.yaml
│   │       ├── helm-values.yaml
│   │       ├── pvc.yaml
│   │       ├── stream-config.yaml
│   │       ├── consumer-config.yaml
│   │       └── README.md
│   │
│   └── monitoring/                     # STORY-1.5
│       ├── kustomization.yaml
│       ├── namespace.yaml
│       ├── prometheus/
│       │   ├── kustomization.yaml
│       │   ├── pvc.yaml
│       │   ├── helm-values.yaml
│       │   ├── servicemonitor.yaml
│       │   └── alerting-rules.yaml
│       ├── grafana/
│       │   ├── kustomization.yaml
│       │   └── dashboards.yaml
│       ├── loki/
│       │   ├── kustomization.yaml
│       │   ├── pvc.yaml
│       │   └── helm-values.yaml
│       └── README.md
```

---

## 部署顺序

### Phase 1: 集群初始化 (STORY-1.1)

```bash
# 1. 以 root 权限运行 k3s 安装脚本
sudo ./scripts/k3s-install.sh

# 2. 验证集群状态
kubectl get nodes
kubectl get namespace
kubectl get resourcequota -A
```

### Phase 2: cert-manager (STORY-1.2)

```bash
cd k8s/platform/cert-manager

# 1. 安装 cert-manager
helm repo add jetstack https://charts.jetstack.io
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.13.3 \
  --set installCRDs=true

# 2. 应用 ClusterIssuer
kubectl apply -f cluster-issuer.yaml

# 3. 测试证书颁发
kubectl apply -f test-certificate.yaml
kubectl wait --for=condition=Ready certificate/test-certificate -n cert-manager --timeout=120s
```

### Phase 3: ArgoCD (STORY-1.3)

```bash
cd k8s/platform/argocd

# 1. 安装 ArgoCD
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/v2.9.3/manifests/install.yaml

# 2. 等待就绪
kubectl wait --for=condition=Ready pods --all -n argocd --timeout=300s

# 3. 配置 RBAC
kubectl apply -f rbac-config.yaml

# 4. 创建 Application
# 注意: 先修改 repoURL 为实际仓库
kubectl apply -f application-gateway.yaml
kubectl apply -f application-services.yaml

# 5. 获取初始密码
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d && echo
```

### Phase 4: NATS JetStream (STORY-1.4)

```bash
cd k8s/platform/nats

# 1. 安装 NATS
helm repo add nats https://nats-io.github.io/k8s/helm/charts/
helm install nats nats/nats \
  --namespace platform \
  --create-namespace \
  --values helm-values.yaml

# 2. 安装 NACK (JetStream Controller)
kubectl apply -f https://raw.githubusercontent.com/nats-io/nack/main/deploy/crds.yml
kubectl apply -f https://raw.githubusercontent.com/nats-io/nack/main/deploy/deployment.yml

# 3. 创建 Stream 和 Consumer
kubectl apply -f stream-config.yaml
kubectl apply -f consumer-config.yaml

# 4. 验证
kubectl get stream -n platform
kubectl get consumer -n platform
```

### Phase 5: 可观测性栈 (STORY-1.5)

```bash
cd k8s/monitoring

# 1. 安装 kube-prometheus-stack
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --values prometheus/helm-values.yaml

# 2. 安装 Loki
helm repo add grafana https://grafana.github.io/helm-charts
helm install loki grafana/loki-stack \
  --namespace monitoring \
  --values loki/helm-values.yaml

# 3. 应用 ServiceMonitor
kubectl apply -f prometheus/servicemonitor.yaml

# 4. 应用告警规则
kubectl apply -f prometheus/alerting-rules.yaml

# 5. 导入 Grafana Dashboard
kubectl apply -f grafana/dashboards.yaml

# 6. 获取 Grafana 密码
kubectl get secret -n monitoring prometheus-grafana \
  -o jsonpath="{.data.admin-password}" | base64 -d && echo
```

---

## 验证清单

### 集群健康

```bash
# 节点状态
kubectl get nodes -o wide

# Namespace 和 ResourceQuota
kubectl get namespace
kubectl get resourcequota -A

# SGX Device Plugin
kubectl get daemonset intel-sgx-plugin -n kube-system
```

### 证书管理

```bash
# cert-manager Pods
kubectl get pods -n cert-manager

# ClusterIssuer
kubectl get clusterissuer

# 测试证书
kubectl get certificate -n cert-manager
```

### GitOps

```bash
# ArgoCD Pods
kubectl get pods -n argocd

# Applications
kubectl get applications -n argocd

# Sync 状态
kubectl describe application neo-services-production -n argocd
```

### 消息队列

```bash
# NATS Pods
kubectl get pods -n platform -l app.kubernetes.io/name=nats

# Stream
kubectl get stream -n platform

# Consumer
kubectl get consumer -n platform
```

### 监控

```bash
# Prometheus
kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090

# Grafana
kubectl get pods -n monitoring -l app.kubernetes.io/name=grafana
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80

# Loki
kubectl get pods -n monitoring -l app.kubernetes.io/name=loki
kubectl port-forward -n monitoring svc/loki 3100:3100
```

---

## 资源分配验证

根据架构文档，验证资源分配:

```bash
# 查看 ResourceQuota 使用情况
kubectl describe resourcequota -n apps
kubectl describe resourcequota -n platform
kubectl describe resourcequota -n monitoring

# 查看节点可分配资源
kubectl describe node | grep -A 10 "Allocatable:"

# 查看 Pod 资源请求
kubectl top pods -n apps
kubectl top pods -n platform
kubectl top pods -n monitoring
```

**预期资源使用**:

- **apps namespace**: 5.5C request / 20Gi memory
- **platform namespace**: 1.5C request / 4Gi memory
- **monitoring namespace**: 1.0C request / 4Gi memory
- **Total**: ~8C / 28Gi (留有系统预留)

---

## 故障排查

### 问题: k3s 安装失败

```bash
# 检查日志
sudo journalctl -u k3s -n 100

# 检查网络
sudo systemctl status firewalld
sudo iptables -L

# 重新安装
sudo /usr/local/bin/k3s-uninstall.sh
sudo ./scripts/k3s-install.sh
```

### 问题: cert-manager 证书不颁发

```bash
# 检查 cert-manager logs
kubectl logs -n cert-manager -l app=cert-manager

# 检查 CertificateRequest
kubectl get certificaterequest -n cert-manager
kubectl describe certificaterequest <name> -n cert-manager
```

### 问题: ArgoCD Application OutOfSync

```bash
# 手动同步
kubectl patch application neo-services-production -n argocd \
  --type merge -p '{"metadata": {"annotations":{"argocd.argoproj.io/refresh": "normal"}}}'

# 查看 sync 详情
kubectl describe application neo-services-production -n argocd
```

### 问题: NATS Stream 无法创建

```bash
# 检查 NACK Controller
kubectl logs -n nats-io -l app=nack

# 手动创建 Stream (通过 nats CLI)
kubectl port-forward -n platform svc/nats 4222:4222
nats stream add neo-events
```

### 问题: Prometheus 无法抓取指标

```bash
# 检查 Targets
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# 访问: http://localhost:9090/targets

# 检查 ServiceMonitor
kubectl get servicemonitor -n monitoring
kubectl describe servicemonitor gateway -n monitoring
```

---

## 下一步 (Sprint 2)

Sprint 1 完成后，继续执行 Sprint 2: TEE Security Foundation

- **STORY-2.1**: MarbleRun Coordinator 生产化
- **STORY-2.2**: Global TEE Signer 服务
- **STORY-2.3**: 30 天密钥自动轮换
- **STORY-2.4**: Header Gate 中间件

---

## 参考文档

- [PRD](/home/ubuntu/service_layer/.claude/specs/neo-devops-refactor/01-product-requirements.md)
- [System Architecture](/home/ubuntu/service_layer/.claude/specs/neo-devops-refactor/02-system-architecture.md)
- [Sprint Plan](/home/ubuntu/service_layer/.claude/specs/neo-devops-refactor/03-sprint-plan.md)
- [cert-manager README](cert-manager/README.md)
- [ArgoCD README](argocd/README.md)
- [NATS README](nats/README.md)
- [Monitoring README](../monitoring/README.md)

---

**Sprint 1 实现完成！** ✅

所有 Story 的配置文件、脚本和文档已经创建。DevOps 工程师可以按照部署顺序逐步执行，建立完整的 k8s 基础设施。
