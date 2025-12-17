# ArgoCD GitOps 设置 (STORY-1.3)

## 概述

ArgoCD 实现 GitOps 工作流，通过 Git 仓库声明式管理所有 Kubernetes 资源。

## 安装步骤

### 1. 安装 ArgoCD

```bash
# 创建 namespace
kubectl create namespace argocd

# 安装 ArgoCD (使用官方 manifest)
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/v2.9.3/manifests/install.yaml

# 等待所有 Pod 就绪
kubectl wait --for=condition=Ready pods --all -n argocd --timeout=300s
```

### 2. 访问 ArgoCD UI

#### 方法 A: 端口转发 (开发环境)

```bash
kubectl port-forward svc/argocd-server -n argocd 8080:443

# 浏览器访问: https://localhost:8080
```

#### 方法 B: Ingress (生产环境)

```bash
# 修改 ingress.yaml 中的域名
# 然后应用配置
kubectl apply -f ingress.yaml

# 访问: https://argocd.yourdomain.com
```

### 3. 获取初始密码

```bash
# 获取 admin 密码
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d && echo

# 登录
# 用户名: admin
# 密码: (上面命令输出的密码)
```

### 4. 修改密码 (推荐)

```bash
# 使用 argocd CLI
argocd account update-password
```

### 5. 配置 RBAC

```bash
kubectl apply -f rbac-config.yaml
```

## Application 配置

### 修改仓库 URL

在以下文件中将 `https://github.com/yourorg/service_layer.git` 替换为实际仓库:

- `application-services.yaml`

### 创建 Application

```bash
# 部署 Neo* Services (选择环境)
kubectl apply -f application-services.yaml
```

## Kustomize Overlay 结构

ArgoCD 使用 Kustomize overlays 管理不同环境:

```
k8s/
├── base/                    # 基础配置
│   ├── kustomization.yaml
│   └── services-deployment.yaml
└── overlays/
    ├── simulation/          # 模拟环境 (无 SGX)
    │   └── kustomization.yaml
    └── production/          # 生产环境 (SGX 硬件)
        └── kustomization.yaml
```

### 环境差异配置

**simulation overlay** (k8s/overlays/simulation/kustomization.yaml):

```yaml
bases:
  - ../../base
patchesStrategicMerge:
  - replica-patch.yaml # 减少副本数
  - resource-patch.yaml # 降低资源限制
```

**production overlay** (k8s/overlays/production/kustomization.yaml):

```yaml
bases:
  - ../../base
patchesStrategicMerge:
  - sgx-patch.yaml # 启用 SGX 资源
  - resource-patch.yaml # 生产资源配置
  - networkpolicy.yaml # 网络策略
```

## 自动同步策略

所有 Application 配置了自动同步:

```yaml
syncPolicy:
  automated:
    prune: true # 自动删除不在 Git 的资源
    selfHeal: true # 自动恢复手动修改
  syncOptions:
    - CreateNamespace=true
```

### 关键配置说明

- **prune**: 删除 Git 中不存在的资源
- **selfHeal**: 检测并恢复 drift (漂移)
- **retry**: 失败时自动重试 (最多 5 次)

## 管理命令

### 查看 Application 状态

```bash
# 列出所有 Application
kubectl get applications -n argocd

# 查看详细状态
kubectl describe application neo-services-production -n argocd
```

### 手动触发同步

```bash
# 使用 kubectl
kubectl patch application neo-services-production -n argocd \
  --type merge -p '{"metadata": {"annotations":{"argocd.argoproj.io/refresh": "normal"}}}'

# 或使用 argocd CLI
argocd app sync neo-services-production
```

### 回滚到历史版本

```bash
# 查看历史
argocd app history neo-services-production

# 回滚到特定版本
argocd app rollback neo-services-production <revision>
```

### 暂停自动同步

```bash
# 暂停
argocd app set neo-services-production --sync-policy none

# 恢复
argocd app set neo-services-production --sync-policy automated
```

## RBAC 权限管理

### 角色定义

1. **admin**: 完全访问 (所有操作)
2. **developer**: 应用管理 (create/update/sync/delete)
3. **viewer**: 只读访问

### 分配角色

编辑 `rbac-config.yaml` 中的 `policy.csv`:

```csv
# 分配 admin 角色给用户
g, alice@example.com, role:admin

# 分配 developer 角色给团队
g, team-dev@example.com, role:developer
```

### OAuth 集成 (可选)

配置 GitHub/Google OAuth:

```yaml
# argocd-cm ConfigMap
data:
  dex.config: |
    connectors:
    - type: github
      id: github
      name: GitHub
      config:
        clientID: $GITHUB_CLIENT_ID
        clientSecret: $GITHUB_CLIENT_SECRET
        orgs:
        - name: yourorg
```

## 监控和告警

### ArgoCD Metrics

ArgoCD 暴露 Prometheus 指标:

```bash
kubectl port-forward svc/argocd-metrics -n argocd 8082:8082
curl http://localhost:8082/metrics
```

关键指标:

- `argocd_app_sync_total`: 同步次数
- `argocd_app_info`: 应用状态
- `argocd_git_request_duration_seconds`: Git 请求延迟

### Grafana Dashboard

导入官方 Dashboard:

- Dashboard ID: 14584 (ArgoCD)

## 故障排查

### Application 一直 Syncing

```bash
# 查看同步详情
argocd app get neo-services-production --refresh

# 查看事件
kubectl describe application neo-services-production -n argocd
```

### Out of Sync 但实际一致

```bash
# 强制刷新
argocd app sync neo-services-production --force

# 忽略差异 (添加到 ignoreDifferences)
```

### Sync 失败

```bash
# 查看详细错误
argocd app sync neo-services-production --dry-run

# 查看日志
kubectl logs -n argocd deployment/argocd-application-controller
```

### Git 仓库无法访问

```bash
# 检查 Secret
kubectl get secret -n argocd | grep repo

# 重新添加仓库
argocd repo add https://github.com/yourorg/service_layer.git \
  --username <user> --password <token>
```

## 最佳实践

### 1. Git 提交规范

- **小步提交**: 每次提交一个逻辑变更
- **描述性消息**: 清晰描述变更内容
- **测试后合并**: 先在 simulation 环境验证

### 2. 环境管理

- **simulation**: 快速迭代，接受失败
- **staging**: 生产前验证 (如果有)
- **production**: 稳定版本，严格测试

### 3. Secrets 管理

- **不要提交 Secret 到 Git**
- 使用 Sealed Secrets 或外部 Secret 管理器:
  - [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets)
  - [External Secrets Operator](https://external-secrets.io/)

### 4. 健康检查

- 配置 `ignoreDifferences` 忽略无关差异
- 使用 `syncOptions: [RespectIgnoreDifferences=true]`
- 定期审查 sync 状态

## 验收标准检查清单

- [ ] ArgoCD 安装并可通过 UI 访问
- [ ] 创建 Application 定义指向服务仓库
- [ ] 配置自动同步策略 (self-heal + prune)
- [ ] 集成 Kustomize overlays (dev/production)
- [ ] 配置 RBAC 限制 ArgoCD 权限
- [ ] Git 提交自动触发部署
- [ ] Sync 失败有明确错误信息

## 参考资料

- [ArgoCD 官方文档](https://argo-cd.readthedocs.io/)
- [Kustomize 文档](https://kustomize.io/)
- [GitOps 最佳实践](https://www.gitops.tech/)
