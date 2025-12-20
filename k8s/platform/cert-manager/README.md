# cert-manager 部署 (STORY-1.2)

## 概述

cert-manager 自动化 TLS 证书管理，支持 Let's Encrypt 和自签名证书。

## 安装步骤

### 1. 使用 Helm 安装 cert-manager

```bash
# 添加 Helm 仓库
helm repo add jetstack https://charts.jetstack.io
helm repo update

# 安装 cert-manager (包含 CRDs)
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.13.3 \
  --set installCRDs=true \
  --set global.leaderElection.namespace=cert-manager \
  --set prometheus.enabled=true \
  --set webhook.timeoutSeconds=30
```

### 2. 验证安装

```bash
# 检查 Pod 状态
kubectl get pods -n cert-manager

# 应看到:
# - cert-manager-xxxxxx          (1/1 Running)
# - cert-manager-cainjector-xxx  (1/1 Running)
# - cert-manager-webhook-xxx     (1/1 Running)

# 检查 CRDs
kubectl get crd | grep cert-manager
```

### 3. 应用 ClusterIssuer

```bash
kubectl apply -f cluster-issuer.yaml
```

**重要**: 修改 `cluster-issuer.yaml` 中的邮箱地址为实际运维邮箱。

### 4. 测试证书颁发

```bash
# 创建测试证书
kubectl apply -f test-certificate.yaml

# 等待证书就绪
kubectl wait --for=condition=Ready certificate/test-certificate -n cert-manager --timeout=120s

# 查看证书详情
kubectl describe certificate test-certificate -n cert-manager

# 验证 Secret 创建
kubectl get secret test-tls-secret -n cert-manager
```

## ClusterIssuer 类型

### 1. selfsigned-issuer (默认)

- **用途**: 开发和测试环境
- **特点**: 无需外部验证，立即颁发
- **限制**: 浏览器不信任

### 2. letsencrypt-staging

- **用途**: 生产前测试
- **特点**: 真实 ACME 流程，但速率限制宽松
- **限制**: 证书不受信任（staging CA）

### 3. letsencrypt-prod

- **用途**: 生产环境
- **特点**: 浏览器信任的证书
- **限制**: 严格速率限制 (50 证书/周/域名)

## Ingress 集成

在 Ingress 中使用证书:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: neofeeds-ingress
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
    - hosts:
        - neofeeds.miniapps.com
      secretName: neofeeds-tls
  rules:
    - host: neofeeds.miniapps.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: neofeeds
                port:
                  number: 8083
```

## 证书续期

cert-manager 自动续期证书:

- 默认在证书过期前 30 天开始续期
- 监控续期状态: `kubectl get certificate -A`
- 查看续期历史: `kubectl describe certificate <name> -n <namespace>`

## 故障排查

### Certificate 一直 Pending

```bash
# 检查 Certificate 事件
kubectl describe certificate <name> -n <namespace>

# 检查 CertificateRequest
kubectl get certificaterequest -n <namespace>

# 检查 Order 和 Challenge (Let's Encrypt)
kubectl get order,challenge -n <namespace>
```

### Webhook 超时

```bash
# 检查 Webhook Pod 日志
kubectl logs -n cert-manager -l app.kubernetes.io/name=webhook

# 重启 Webhook
kubectl rollout restart deployment cert-manager-webhook -n cert-manager
```

### Let's Encrypt 速率限制

- **速率限制触发**: 使用 staging issuer 测试
- **等待重置**: 速率限制按周重置
- **多域名**: 使用通配符证书减少请求

## 监控

cert-manager 暴露 Prometheus 指标:

```bash
kubectl port-forward -n cert-manager svc/cert-manager 9402:9402
curl http://localhost:9402/metrics
```

关键指标:

- `certmanager_certificate_expiration_timestamp_seconds`: 证书过期时间
- `certmanager_certificate_ready_status`: 证书就绪状态

## 验收标准检查清单

- [ ] cert-manager CRDs 和 Controller 部署成功
- [ ] 配置 Let's Encrypt ClusterIssuer
- [ ] 测试证书自动颁发
- [ ] 证书自动续期功能验证
- [ ] Webhook 健康检查正常

## 参考资料

- [cert-manager 官方文档](https://cert-manager.io/docs/)
- [Let's Encrypt 速率限制](https://letsencrypt.org/docs/rate-limits/)
- [ACME HTTP-01 Challenge](https://cert-manager.io/docs/configuration/acme/http01/)
