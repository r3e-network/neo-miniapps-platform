# 可观测性栈部署 (STORY-1.5)

## 概述

部署 Prometheus、Grafana 和 Loki 组成完整的可观测性栈，实现指标监控、可视化和日志聚合。

## 组件架构

```
┌─────────────────────────────────────────────────────────────┐
│                    Observability Stack                       │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │  Prometheus  │  │   Grafana    │  │     Loki     │     │
│  │  (Metrics)   │─▶│ (Dashboards) │◀─│    (Logs)    │     │
│  └──────┬───────┘  └──────────────┘  └───────▲──────┘     │
│         │                                     │              │
│         │ /metrics                            │ logs         │
│         ▼                                     │              │
│  ┌────────────────────────────────────────────┴────────┐    │
│  │            Neo* Services (apps namespace)          │    │
│  └──────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## 安装步骤

### 1. 安装 kube-prometheus-stack (包含 Prometheus + Grafana)

```bash
# 添加 Helm 仓库
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# 创建 namespace
kubectl create namespace monitoring

# 安装
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values prometheus/helm-values.yaml

# 等待所有 Pod 就绪
kubectl wait --for=condition=Ready pods --all -n monitoring --timeout=300s
```

### 2. 安装 Loki (日志聚合)

```bash
# 添加 Grafana Helm 仓库
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

# 创建 PVC
kubectl apply -f loki/pvc.yaml

# 安装 Loki Stack (包含 Promtail)
helm install loki grafana/loki-stack \
  --namespace monitoring \
  --values loki/helm-values.yaml
```

### 3. 应用 ServiceMonitor (自动发现服务指标)

```bash
kubectl apply -f prometheus/servicemonitor.yaml
```

### 4. 应用告警规则

```bash
kubectl apply -f prometheus/alerting-rules.yaml
```

### 5. 导入 Grafana Dashboards

```bash
kubectl apply -f grafana/dashboards.yaml
```

## 访问 Grafana UI

### 方法 A: 端口转发 (开发环境)

```bash
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80

# 浏览器访问: http://localhost:3000
# 用户名: admin
# 密码: 从 Kubernetes Secret 获取 (见下方"获取初始密码"部分)
#
# ⚠️ 重要: 部署前必须先创建 grafana-admin Secret:
# kubectl create secret generic grafana-admin \
#   --namespace monitoring \
#   --from-literal=admin-user=admin \
#   --from-literal=admin-password=$(openssl rand -base64 32)
```

### 方法 B: Ingress (生产环境)

Grafana Ingress 已在 `prometheus/helm-values.yaml` 中配置:

- 访问: https://grafana.local (需配置 DNS 或 /etc/hosts)

### 获取初始密码

```bash
kubectl get secret -n monitoring prometheus-grafana \
  -o jsonpath="{.data.admin-password}" | base64 -d && echo
```

## 验证安装

### 检查 Prometheus

```bash
# 检查 Pod 状态
kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus

# 检查 Targets (应看到所有服务)
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# 浏览器访问: http://localhost:9090/targets
```

### 检查 Grafana

```bash
# 检查 Pod 状态
kubectl get pods -n monitoring -l app.kubernetes.io/name=grafana

# 验证数据源
# Grafana UI → Configuration → Data Sources
# 应看到 Prometheus 和 Loki 数据源
```

### 检查 Loki

```bash
# 检查 Pod 状态
kubectl get pods -n monitoring -l app.kubernetes.io/name=loki

# 测试日志查询
kubectl port-forward -n monitoring svc/loki 3100:3100
curl -G http://localhost:3100/loki/api/v1/label | jq
```

## ServiceMonitor 自动发现

Prometheus Operator 使用 ServiceMonitor CRD 自动发现和抓取指标。

### 为服务添加 ServiceMonitor

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: my-service
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: my-service
  namespaceSelector:
    matchNames:
      - apps
  endpoints:
    - port: metrics
      path: /metrics
      interval: 30s
```

### 验证 ServiceMonitor

```bash
# 列出所有 ServiceMonitor
kubectl get servicemonitor -n monitoring

# 查看 Prometheus 配置
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# 访问: http://localhost:9090/config
```

## 告警规则

告警规则根据 PRD 非功能需求配置:

- **CPU > 80%**: Warning (5 分钟)
- **CPU > 95%**: Critical (2 分钟)
- **内存 > 85%**: Warning (5 分钟)
- **错误率 > 5%**: Warning (5 分钟)

### 查看告警

```bash
# Prometheus UI → Alerts
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# 访问: http://localhost:9090/alerts

# Alertmanager UI
kubectl port-forward -n monitoring svc/alertmanager-operated 9093:9093
# 访问: http://localhost:9093
```

### 配置 Alertmanager 接收器

编辑 `prometheus/helm-values.yaml` 中的 `alertmanager.config.receivers`:

```yaml
receivers:
  - name: "slack"
    slack_configs:
      - api_url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
        channel: "#alerts"
        title: "{{ .GroupLabels.alertname }}"
        text: "{{ .CommonAnnotations.description }}"

  - name: "email"
    email_configs:
      - to: "ops@miniapps.com"
        from: "alertmanager@miniapps.com"
        smarthost: "smtp.miniapps.com:587"
        auth_username: "alertmanager"
        # ⚠️ 生产环境: 使用 Secret 管理凭据，不要硬编码密码
        # auth_password 应从 Secret 读取或使用 auth_password_file
        auth_password: "<从Secret获取>"
```

## Grafana Dashboard

### 预配置 Dashboard

1. **NEO Service Layer Overview** (自定义)
   - CPU/内存使用率
   - HTTP 请求速率和错误率
   - NATS JetStream 消息数

2. **Kubernetes Cluster** (ID: 7249)
   - 节点资源使用
   - Pod 状态
   - Namespace 资源配额

3. **NATS JetStream** (ID: 14892)
   - Stream 消息数
   - Consumer 延迟
   - 连接数

4. **Node Exporter** (ID: 1860)
   - 节点 CPU/内存/磁盘
   - 网络流量

### 创建自定义 Dashboard

1. Grafana UI → Dashboards → New Dashboard
2. Add Panel → 选择 Prometheus 数据源
3. 输入 PromQL 查询:

   ```promql
   # CPU 使用率
   sum(rate(container_cpu_usage_seconds_total{namespace="service-layer"}[5m])) by (pod)

   # 内存使用率
   sum(container_memory_working_set_bytes{namespace="service-layer"}) by (pod)

   # HTTP 请求速率
   sum(rate(http_requests_total{namespace="service-layer"}[5m])) by (service)
   ```

## Loki 日志查询

### LogQL 查询语法

```bash
# 查询特定 namespace 的日志
{namespace="service-layer"}

# 过滤包含 "error" 的日志
{namespace="service-layer"} |= "error"

# 查询特定 Pod
{namespace="service-layer", pod=~"neofeeds-.*"}

# 正则表达式过滤
{namespace="service-layer"} |~ "error|fail|exception"

# JSON 解析
{namespace="service-layer"} | json | level="error"
```

### Grafana 中查询日志

1. Grafana UI → Explore
2. 选择 Loki 数据源
3. 输入 LogQL 查询
4. 时间范围选择

## 关键指标 PromQL

### 服务健康

```promql
# Pod 就绪状态
sum(kube_pod_status_ready{namespace="service-layer", condition="true"}) by (pod)

# Pod 重启次数
rate(kube_pod_container_status_restarts_total{namespace="service-layer"}[15m])
```

### 资源使用

```promql
# CPU 使用率 (%)
100 * sum(rate(container_cpu_usage_seconds_total{namespace="service-layer"}[5m])) by (pod)
/ sum(container_spec_cpu_quota{namespace="service-layer"} / container_spec_cpu_period{namespace="service-layer"}) by (pod)

# 内存使用率 (%)
100 * sum(container_memory_working_set_bytes{namespace="service-layer"}) by (pod)
/ sum(container_spec_memory_limit_bytes{namespace="service-layer"}) by (pod)
```

### HTTP 性能

```promql
# P99 延迟
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service))

# 请求速率
sum(rate(http_requests_total[5m])) by (service)

# 错误率 (%)
100 * sum(rate(http_requests_total{status=~"5.."}[5m])) by (service)
/ sum(rate(http_requests_total[5m])) by (service)
```

### NATS JetStream

```promql
# Stream 消息数
nats_jetstream_stream_messages{stream_name="neo-events"}

# Consumer 延迟
nats_jetstream_consumer_num_pending

# 消息处理速率
rate(nats_jetstream_consumer_delivered_consumer_seq[5m])
```

## 故障排查

### Prometheus 无法抓取指标

```bash
# 检查 ServiceMonitor
kubectl get servicemonitor -n monitoring

# 检查 Targets
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# 访问: http://localhost:9090/targets
# 查看 "State" 列，如果是 "Down" 则有问题

# 检查服务是否暴露 /metrics
kubectl exec -it <pod-name> -n apps -- curl localhost:8080/metrics
```

### Grafana 无法连接 Prometheus

```bash
# 检查 Prometheus Service
kubectl get svc -n monitoring | grep prometheus

# 测试连接
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://prometheus-operated.monitoring.svc.cluster.local:9090/api/v1/status/config
```

### Loki 无法接收日志

```bash
# 检查 Promtail Pod
kubectl get pods -n monitoring -l app.kubernetes.io/name=promtail

# 查看 Promtail 日志
kubectl logs -n monitoring -l app.kubernetes.io/name=promtail

# 测试 Loki API
kubectl exec -it loki-0 -n monitoring -- \
  curl http://localhost:3100/ready
```

### 告警不触发

```bash
# 检查告警规则
kubectl get prometheusrule -n monitoring

# 查看 Prometheus 规则状态
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# 访问: http://localhost:9090/rules

# 检查 Alertmanager 配置
kubectl get secret -n monitoring alertmanager-prometheus-kube-prometheus-alertmanager \
  -o jsonpath='{.data.alertmanager\.yaml}' | base64 -d
```

## 性能优化

### Prometheus 存储优化

```yaml
# 调整保留策略
prometheus:
  prometheusSpec:
    retention: 15d
    retentionSize: "9GB"

    # TSDB 压缩
    compaction:
      enabled: true
```

### Loki 查询性能

```yaml
# 增加查询并行度
loki:
  config:
    querier:
      max_concurrent: 4
    query_range:
      parallelise_shardable_queries: true
```

## 验收标准检查清单

- [ ] Prometheus 采集所有服务 `/metrics` 端点
- [ ] Grafana 仪表盘展示关键 SLI (CPU/内存/请求率/错误率)
- [ ] Loki 聚合所有容器日志 (通过 Promtail)
- [ ] 配置告警规则 (CPU > 80%, Memory > 85%, Error Rate > 5%)
- [ ] 测试分布式追踪 (OpenTelemetry collector)

## 参考资料

- [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)
- [Prometheus Operator](https://prometheus-operator.dev/)
- [Grafana 文档](https://grafana.com/docs/)
- [Loki 文档](https://grafana.com/docs/loki/latest/)
- [PromQL 教程](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [LogQL 教程](https://grafana.com/docs/loki/latest/logql/)
