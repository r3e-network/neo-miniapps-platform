# NATS JetStream 部署 (STORY-1.4)

## 概述

NATS JetStream 提供持久化消息队列，支持异步事件处理和 at-least-once 交付保证。

## 架构决策

根据架构文档 Section 3.2：

- **Stream 设计**: 单一 `neo-events` stream，使用 subject 过滤
- **Subjects**: `neo.vrf.*`, `neo.vault.*`, `neo.datafeed.*`, 等
- **保留策略**: 7 天或 10GB (先达到者)
- **存储**: 文件存储 (PVC 5Gi)

## 安装步骤

### 1. 安装 NATS Helm Chart

```bash
# 添加 Helm 仓库
helm repo add nats https://nats-io.github.io/k8s/helm/charts/
helm repo update

# 创建 PVC
kubectl apply -f pvc.yaml

# 安装 NATS with JetStream
helm install nats nats/nats \
  --namespace platform \
  --create-namespace \
  --values helm-values.yaml

# 等待 Pod 就绪
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=nats -n platform --timeout=120s
```

### 2. 安装 NACK (NATS JetStream Controller)

NACK 允许通过 CRD 管理 Stream 和 Consumer:

```bash
# 安装 CRDs
kubectl apply -f https://raw.githubusercontent.com/nats-io/nack/main/deploy/crds.yml

# 安装 Controller
kubectl apply -f https://raw.githubusercontent.com/nats-io/nack/main/deploy/deployment.yml

# 验证安装
kubectl get crd | grep jetstream
```

### 3. 创建 Stream 和 Consumer

```bash
# 应用 Stream 配置
kubectl apply -f stream-config.yaml

# 应用 Consumer 配置
kubectl apply -f consumer-config.yaml

# 验证创建
kubectl get stream -n platform
kubectl get consumer -n platform
```

## 验证安装

### 检查 Pod 状态

```bash
kubectl get pods -n platform -l app.kubernetes.io/name=nats
```

### 检查 Stream 信息

```bash
# 使用 nats CLI (需要安装)
kubectl port-forward -n platform svc/nats 4222:4222

# 在另一个终端
nats stream ls
nats stream info neo-events
```

### 测试消息发布

```bash
# 发布测试消息
nats pub neo.vrf.request '{"requestId":"test-001","seed":"abc123"}'

# 查看消息
nats stream view neo-events
```

## Go 客户端使用示例

### 连接 NATS

```go
package main

import (
    "github.com/nats-io/nats.go"
    "log"
)

func main() {
    // 连接到 NATS
    nc, err := nats.Connect("nats://nats.platform.svc.cluster.local:4222")
    if err != nil {
        log.Fatal(err)
    }
    defer nc.Close()

    // 获取 JetStream Context
    js, err := nc.JetStream()
    if err != nil {
        log.Fatal(err)
    }

    // 发布消息
    ack, err := js.Publish("neo.vrf.request", []byte(`{"requestId":"vrf-001"}`))
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Published: %+v\n", ack)
}
```

### 订阅消息 (Pull Consumer)

```go
func consumeMessages(js nats.JetStreamContext) {
    // 订阅 Consumer
    sub, err := js.PullSubscribe(
        "neo.vrf.*",
        "neorand-consumer",
        nats.ManualAck(),
    )
    if err != nil {
        log.Fatal(err)
    }

    for {
        // 拉取消息 (批量)
        msgs, err := sub.Fetch(10, nats.MaxWait(5*time.Second))
        if err != nil {
            // 超时或其他错误
            continue
        }

        for _, msg := range msgs {
            log.Printf("Received: %s\n", string(msg.Data))

            // 处理消息
            if err := processMessage(msg.Data); err != nil {
                msg.Nak()  // Negative Ack (重试)
            } else {
                msg.Ack()  // 确认
            }
        }
    }
}
```

### 订阅消息 (Push Consumer)

```go
func subscribePush(js nats.JetStreamContext) {
    // Push Consumer (自动推送)
    sub, err := js.Subscribe(
        "neo.vrf.*",
        func(msg *nats.Msg) {
            log.Printf("Received: %s\n", string(msg.Data))

            // 处理消息
            if err := processMessage(msg.Data); err != nil {
                msg.Nak()
            } else {
                msg.Ack()
            }
        },
        nats.Durable("neorand-consumer"),
        nats.ManualAck(),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer sub.Unsubscribe()

    // 阻塞等待消息
    select {}
}
```

### 幂等性保证

```go
type MessageProcessor struct {
    db *sql.DB
}

func (p *MessageProcessor) Process(msg *nats.Msg) error {
    // 解析消息
    var event VRFRequest
    if err := json.Unmarshal(msg.Data, &event); err != nil {
        return err
    }

    // 检查是否已处理 (幂等性)
    var count int
    err := p.db.QueryRow(
        "SELECT COUNT(*) FROM processed_requests WHERE request_id = $1",
        event.RequestID,
    ).Scan(&count)
    if err != nil {
        return err
    }

    if count > 0 {
        log.Printf("Request %s already processed, skipping\n", event.RequestID)
        return nil
    }

    // 处理业务逻辑
    if err := p.processVRF(event); err != nil {
        return err
    }

    // 标记为已处理
    _, err = p.db.Exec(
        "INSERT INTO processed_requests (request_id, processed_at) VALUES ($1, NOW())",
        event.RequestID,
    )
    return err
}
```

## Stream 和 Consumer 概念

### Stream

- **定义**: 消息存储单元，持久化消息
- **Subject**: 消息的路由键 (类似 MQTT Topic)
- **Retention**: 保留策略 (时间、大小、消息数)

### Consumer

- **定义**: 消息消费者，跟踪消费进度
- **Durable**: 持久化消费者，重启后恢复进度
- **DeliverPolicy**: 交付策略 (all, last, new, by_start_sequence)
- **AckPolicy**: 确认策略 (explicit, none, all)

### Subject 通配符

- `neo.*`: 匹配单层 (如 `neo.vrf`)
- `neo.>`: 匹配多层 (如 `neo.vrf.request`, `neo.vrf.fulfilled`)

## 监控

### Prometheus 指标

```bash
kubectl port-forward -n platform svc/nats 7777:7777
curl http://localhost:7777/metrics
```

关键指标:

- `nats_jetstream_stream_messages`: Stream 中的消息数
- `nats_jetstream_stream_bytes`: Stream 占用字节数
- `nats_jetstream_consumer_delivered`: Consumer 交付的消息数
- `nats_jetstream_consumer_ack_pending`: 待确认的消息数

### Grafana Dashboard

导入 NATS 官方 Dashboard:

- [NATS JetStream Dashboard](https://grafana.com/grafana/dashboards/14892)

## 故障排查

### Stream 创建失败

```bash
# 查看 Stream 状态
kubectl describe stream neo-events -n platform

# 查看 NACK Controller 日志
kubectl logs -n nats-io -l app=nack
```

### Consumer 无法消费

```bash
# 检查 Consumer 配置
kubectl get consumer neorand-consumer -n platform -o yaml

# 使用 nats CLI 调试
nats consumer info neo-events neorand-consumer
```

### 消息积压

```bash
# 查看 pending 消息数
nats stream info neo-events

# 增加 Consumer 并行度或优化处理速度
```

### PVC 空间不足

```bash
# 检查 PVC 使用率
kubectl exec -n platform -it nats-0 -- df -h

# 扩容 PVC (需要 StorageClass 支持动态扩容)
kubectl edit pvc nats-jetstream-pvc -n platform
# 修改 spec.resources.requests.storage
```

## 性能优化

### Batch 发布

```go
// 批量发布减少 RTT
var futures []nats.PubAckFuture
for _, event := range events {
    data, _ := json.Marshal(event)
    future, _ := js.PublishAsync("neo.vrf.request", data)
    futures = append(futures, future)
}

// 等待所有完成
for _, future := range futures {
    <-future.Ok()
}
```

### 并行消费

```go
// 启动多个 goroutine 并行消费
for i := 0; i < 4; i++ {
    go func() {
        consumeMessages(js)
    }()
}
```

### DLQ (Dead Letter Queue)

```go
func processWithDLQ(msg *nats.Msg, js nats.JetStreamContext) {
    if err := processMessage(msg.Data); err != nil {
        // 超过最大重试次数，发送到 DLQ
        meta, _ := msg.Metadata()
        if meta.NumDelivered >= 3 {
            js.Publish("neo.dlq.vrf", msg.Data)
            msg.Ack()  // 确认原消息
        } else {
            msg.NakWithDelay(5 * time.Second)  // 延迟重试
        }
    } else {
        msg.Ack()
    }
}
```

## 验收标准检查清单

- [ ] NATS Server 和 JetStream 启用
- [ ] 配置持久化存储 (PVC 5Gi)
- [ ] 创建 Stream `neo-events` 和 Consumer
- [ ] 测试消息持久化和重放功能
- [ ] Go 客户端示例代码可运行

## 参考资料

- [NATS JetStream 文档](https://docs.nats.io/nats-concepts/jetstream)
- [NACK (JetStream Controller)](https://github.com/nats-io/nack)
- [nats.go 客户端](https://github.com/nats-io/nats.go)
- [JetStream 设计文档](https://github.com/nats-io/jetstream)
