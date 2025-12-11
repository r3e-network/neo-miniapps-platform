# Neo Service Layer 功能文档

## 📋 目录

1. [项目概述](#1-项目概述)
2. [技术架构](#2-技术架构)
3. [核心组件](#3-核心组件)
4. [服务详解](#4-服务详解)
5. [安全模型](#5-安全模型)
6. [数据流程](#6-数据流程)
7. [部署指南](#7-部署指南)
8. [API 参考](#8-api-参考)
9. [NeoVault 安全与争议处理模型](#9-neovault-安全与争议处理模型)
10. [服务升级安全性](#10-服务升级安全性)

---

## 1. 项目概述

### 1.1 简介

Neo Service Layer 是一个基于可信执行环境 (TEE) 的生产级服务平台，为 Neo N3 区块链提供安全、可验证的链下计算服务。

### 1.2 核心技术栈

| 组件 | 技术 | 用途 |
|------|------|------|
| **机密计算** | MarbleRun + EGo | MarbleRun TEE 编排和 Go 运行时 |
| **数据库** | Supabase (PostgreSQL) | 持久化存储 + RLS 安全策略 |
| **前端托管** | Netlify | 静态站点部署 + CDN |
| **编程语言** | Go 1.21+ | 后端服务实现 |

### 1.3 设计原则

- **纵深防御**: 每一层都提供安全保护
- **零信任架构**: 所有组件间通过证明验证身份
- **最小攻击面**: 服务能力受 manifest 严格限制
- **密钥永不离开 TEE**: 使用回调模式访问敏感数据

---

## 2. 技术架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           客户端层                                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                      │
│  │   Web App   │  │  Mobile App │  │   DApp      │                      │
│  │  (Netlify)  │  │             │  │             │                      │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘                      │
└─────────┼────────────────┼────────────────┼─────────────────────────────┘
          │                │                │
          └────────────────┼────────────────┘
                           │ HTTPS
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        API 网关层 (Gateway Marble)                       │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │  • JWT 认证验证                                                   │   │
│  │  • 请求路由分发                                                   │   │
│  │  • 速率限制                                                       │   │
│  │  • TLS 终止 (在 TEE 内)                                       │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
                           │ mTLS
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        服务层 (Service Marbles)                          │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐            │
│  │   VRF   │ │  NeoVault  │ │NeoFeeds│ │Automate │ │Confiden │            │
│  └─────────┘ └────┬────┘ └─────────┘ └─────────┘ └─────────┘            │
│                   │ HTTP (内部)                                          │
│                   ▼                                                     │
│            ┌─────────────┐                                              │
│            │ AccountPool │  ← 内部服务: 账户池管理、HD密钥派生、交易签名       │
│            │  (Internal) │    私钥永不离开此服务                           │
│            └─────────────┘                                              │
│                                                                         │
│  注: GasBank 为核心基础设施 (internal/gasbank)，非独立服务                   │
└─────────────────────────────────────────────────────────────────────────┘
                           │ mTLS
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                     MarbleRun Coordinator                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │  Manifest   │  │   Secrets   │  │ Attestation │  │     PKI     │     │
│  │   Store     │  │    Store    │  │   Engine    │  │   Manager   │     │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘     │
└─────────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        数据层 (Supabase)                                 │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  PostgreSQL + Row Level Security (RLS)                           │   │
│  │  • users, api_keys, secrets                                      │   │
│  │  • service_requests, price_feeds                                 │   │
│  │  • gasbank_accounts, neoflow_triggers                         │   │
│  │  • vrf_requests, neovault_requests, pool_accounts                    │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.2 MarbleRun + EGo 集成

#### 2.2.1 Marble 生命周期

```
1. Marble 在 EGo TEE 内启动
         │
         ▼
2. Marble 生成远程证明报告 (attestation quote)
         │
         ▼
3. Marble 连接到 Coordinator
         │
         ▼
4. Coordinator 验证证明报告是否符合 manifest
         │
         ▼
5. Coordinator 注入密钥和 TLS 证书
         │
         ▼
6. Marble 开始处理请求
```

#### 2.2.2 Marble SDK 核心结构

```go
// internal/marble/marble.go
type Marble struct {
    // 身份标识
    marbleType string
    uuid       string

    // TLS 凭证 (由 Coordinator 注入)
    cert       tls.Certificate
    rootCA     *x509.CertPool
    tlsConfig  *tls.Config

    // 密钥 (由 Coordinator 注入)
    secrets    map[string][]byte

    // TEE 报告
    report     *attestation.Report
}
```

#### 2.2.3 密钥安全访问模式

```go
// 密钥永不离开 TEE - 只能通过回调访问
err := marble.UseSecret("API_KEY", func(secret []byte) error {
    // 在这里使用密钥
    // 回调返回后自动清零
    return doSomethingWithSecret(secret)
})
```

### 2.3 Supabase 集成

#### 2.3.1 数据库架构

```sql
-- 核心表结构
users                  -- 用户账户
api_keys               -- API 密钥
user_wallets           -- 用户钱包
user_sessions          -- 用户会话
service_requests       -- 服务请求记录
price_feeds            -- 价格数据
gasbank_accounts       -- Gas 银行账户
gasbank_transactions   -- Gas 银行交易记录
deposit_requests       -- 充值请求
neoflow_triggers    -- 自动化触发器
neoflow_executions  -- 自动化执行记录
vrf_requests           -- VRF 请求
pool_accounts          -- 共享账户池 (由 AccountPool 服务管理)
neovault_requests         -- 混币请求记录
```

#### 2.3.2 Row Level Security (RLS)

```sql
-- 启用 RLS
ALTER TABLE secrets ENABLE ROW LEVEL SECURITY;

-- 服务角色可访问所有数据
CREATE POLICY service_all ON secrets
    FOR ALL TO service_role USING (true);
```

### 2.4 Netlify 前端部署

```toml
# frontend/netlify.toml
[build]
  command = "npm run build"
  publish = "dist"

[[redirects]]
  from = "/*"
  to = "/index.html"
  status = 200

[build.environment]
  NODE_VERSION = "20"

[[headers]]
  for = "/*"
  [headers.values]
    X-Frame-Options = "DENY"
    X-Content-Type-Options = "nosniff"
    Referrer-Policy = "strict-origin-when-cross-origin"
```

---

## 3. 核心组件

### 3.1 目录结构

```
service_layer/
├── cmd/
│   ├── gateway/main.go      # API 网关入口
│   └── marble/main.go       # 通用 Marble 入口
├── internal/
│   ├── crypto/              # 加密工具库
│   │   ├── crypto.go        # AES-GCM, ECDSA, VRF, HKDF, HMAC
│   │   └── crypto_test.go
│   ├── marble/              # Marble SDK
│   │   ├── marble.go        # Marble 核心实现
│   │   └── service.go       # 服务基类
│   ├── database/            # 数据库层
│   │   ├── supabase.go      # Supabase 客户端
│   │   └── supabase_test.go
│   ├── gasbank/             # 余额管理 (核心基础设施)
│   │   ├── gasbank.go       # 存款、提款、费用管理
│   │   └── gasbank_test.go
│   └── chain/               # Neo N3 链交互
│       └── client.go
├── services/                # 服务实现
│   ├── vrf/                 # 可验证随机函数
│   ├── neovault/               # 隐私混币
│   ├── accountpool/         # 内部服务: 账户池管理、HD密钥、签名
│   ├── neofeeds/           # 价格数据聚合
│   ├── neoflow/          # 自动化触发器
│   └── neocompute/        # 机密计算 (规划中)
├── manifests/
│   └── manifest.json        # MarbleRun manifest
├── migrations/
│   └── 001_initial_schema.sql
├── docker/
│   ├── docker-compose.yaml
│   ├── Dockerfile.gateway
│   └── Dockerfile.service
└── frontend/                # React 前端
```

### 3.2 服务基类

```go
// internal/marble/service.go
type Service struct {
    id      string
    name    string
    version string
    marble  *Marble
    db      *database.Repository
    router  *mux.Router
}

// 每个服务都嵌入基类
type VRFService struct {
    *marble.Service
    privateKey []byte
}
```

### 3.3 加密工具库

| 功能 | 函数 | 说明 |
|------|------|------|
| 密钥派生 | `DeriveKey()` | HKDF-SHA256 |
| 对称加密 | `Encrypt()/Decrypt()` | AES-256-GCM |
| 签名 | `Sign()/Verify()` | ECDSA P-256 |
| VRF | `GenerateVRF()/VerifyVRF()` | 可验证随机函数 |
| 哈希 | `Hash256()/Hash160()` | SHA256, RIPEMD160 |
| Neo 地址 | `ScriptHashToAddress()` | Neo N3 地址生成 |
| HMAC | `HMACSign()/HMACVerify()` | HMAC-SHA256 签名验证 |

### 3.4 余额管理 (GasBank)

**位置**: `internal/gasbank/` (核心基础设施，非独立服务)

**功能**: 管理用户 Gas 费用账户

**核心操作**:
| 操作 | 函数 | 说明 |
|------|------|------|
| 存款 | `Deposit()` | 充值到用户账户 |
| 提款 | `Withdraw()` | 从用户账户提取 |
| 预留 | `Reserve()` | 服务执行前预留费用 |
| 释放 | `Release()` | 服务失败时释放预留 |
| 消费 | `Consume()` | 服务成功后扣除预留 |
| 直接扣费 | `ChargeServiceFee()` | 同步扣除服务费 |

**费用标准**:
| 服务 | 费用 (GAS) |
|------|------|
| VRF | 0.001 |
| NeoFlow | 0.0005 |
| NeoFeeds | 0.0001 |
| NeoVault | 0.05 + 0.5% |
| NeoCompute | 0.001 |

---

## 4. 服务详解

### 4.1 VRF 服务 (可验证随机函数)

**功能**: 生成可验证的随机数

**工作流程**:
```
1. 客户端提交种子 (seed)
         │
         ▼
2. VRF 使用私钥生成证明
         │
         ▼
3. 从证明派生随机数
         │
         ▼
4. 返回随机数 + 证明 + 公钥
         │
         ▼
5. 任何人可用公钥验证
```

**API 端点**:
```
POST /neorand/random
{
    "seed": "0x1234...",
    "num_words": 3
}

Response:
{
    "seed": "0x1234...",
    "random_words": ["0xabc...", "0xdef...", "0x123..."],
    "proof": "0x...",
    "public_key": "0x...",
    "timestamp": "2024-12-06T12:00:00Z"
}

POST /neorand/verify
{
    "seed": "0x1234...",
    "random_words": [...],
    "proof": "0x...",
    "public_key": "0x..."
}

Response:
{
    "valid": true
}
```

### 4.2 NeoVault 服务 (隐私混币，账户模型 + TEE + Dispute)

#### 4.2.1 总体目标

NeoVault 服务在 Neo N3 账户模型下提供一种 **隐私增强的资产流转方式**，通过：

- 由 TEE 控制的一组 **HD 派生池地址** 作为中转和混淆节点
- 链下混币逻辑运行在 TEE（EGo + MarbleRun）中
- **用户直接给池地址转账**，而不是先通过链上 NeoVault 合约请求
- 使用 **请求哈希 (request hash) + TEE 签名** 为每次混币建立可审计凭证
- 在正常路径下尽量提供隐私混淆
- 在**超时或争议时放弃隐私，用链上 Dispute 机制换取补偿**

#### 隐私优先费用模型

**用户永远不会直接连接到任何已知的服务层账户。** 这通过以下方式实现：

1. **直接池存款**: 用户直接存款到匿名池账户（不是 gasbank，不是任何已知的服务层地址）
2. **隐式费用收取**: 费用从交付金额中扣除（用户收到 `净金额 = 总金额 - 服务费`）
3. **无费用交易**: 服务费保留在池账户中分布 - 没有显式的费用转账到任何已知地址
4. **零链上关联**: 没有交易将用户连接到服务层身份

```
用户存款:  100 GAS  →  池账户 (匿名)
用户收到:  99.5 GAS →  目标地址
费用保留:  0.5 GAS  →  留在池中 (无显式转账)
```

#### 4.2.2 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                     链下 TEE (NeoVault Marble)                      │
│                                                                 │
│  • 持有 HD 种子 (由 MarbleRun 注入的 Master_Secret)                │
│  • 派生池地址私钥 (pool_0, pool_1, ..., pool_N)                    │
│  • 接收用户混币请求 (链下 API/CLI)                                  │
│  • 生成 requestHash + TEE 签名                                    │
│  • 执行入池交易广播 / 池内混淆 / 出池交易                             │
│  • 记录每个 requestId 所对应的资金流                                 │
│                                                                  │
└───────────────────┬──────────────────────────────────────────────┘
                    │
                    │（超时争议时）
                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                NeoVault 合约 (同时承担 Dispute 职责)                  │
│                                                                 │
│  • 记录每个 requestHash 的状态：                                   │
│    - deadline (超时时间)                                         │
│    - fulfilled (是否已证明完成)                                   │
│    - disputed (是否进入争议流程)                                   │
│  • 验证用户提交的：                                                │
│    - 原始申请字段 (requestBytes)                                  │
│    - requestHash                                                │
│    - TEE 对 requestHash 的签名 (sig_TEE)                         │
│  • 验证 NeoVault 提交的履约证明：                                     │
│    - TEE 对 (requestHash || txidsHash) 的签名                    │
│  • 超时且未履约 → 允许用户从保证金池领赔付                            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

#### 4.2.3 账户模型下的池地址架构

与 UTXO 不同，Neo N3 使用账户模型。NeoVault 的资金流：

- 若干由 **AccountPool 内部服务** 管理的 **池地址**（Pool Accounts），均为普通单签地址：
  - 从链上看，这些地址与普通用户地址在脚本/类型上无法区分
  - 但其行为模式可以被一定程度分析（这是账户模型的天然限制）
  - **私钥永不离开 AccountPool 服务**，其他服务只能请求签名
- 用户直接向这些池地址转账（链上表现为普通转账）
- NeoVault 服务通过 HTTP 调用 AccountPool 请求账户、构造交易、请求签名

**AccountPool 服务架构：**

```
┌─────────────────────────────────────────────────────────────────┐
│                   AccountPool Service (内部)                     │
│                                                                 │
│  • 持有 HD 种子 (POOL_MASTER_KEY，由 MarbleRun 注入)              │
│  • 派生池地址私钥，私钥永不离开此服务                                │
│  • 提供 HTTP API 供其他服务调用：                                  │
│    - POST /request  - 请求并锁定账户                              │
│    - POST /release  - 释放账户回池                                │
│    - POST /sign     - 签名交易 (仅限锁定该账户的服务)                │
│    - POST /balance  - 更新账户余额                                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ HTTP (mTLS)
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     NeoVault Service                               │
│                                                                 │
│  • 请求池账户用于混币                                              │
│  • 构造交易，请求 AccountPool 签名                                 │
│  • 混币完成后释放账户                                              │
│  • 不直接持有任何池账户私钥                                         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

池地址通过 HD 种子确定性派生 (在 AccountPool 内部)：

```
POOL_MASTER_KEY (由 Coordinator 注入到 AccountPool)
  │
  ├── HKDF(masterKey, accountID_0, "pool-account") → PrivKey_pool0 → Addr_pool0
  ├── HKDF(masterKey, accountID_1, "pool-account") → PrivKey_pool1 → Addr_pool1
  └── ...
```

**账户锁定机制：**

- 服务请求账户时，AccountPool 锁定账户并记录 `locked_by` 字段
- 只有锁定账户的服务才能请求签名
- 账户有 24 小时锁定超时，防止僵尸锁
- 混币完成后 NeoVault 显式释放账户

**合规性配置：**

- 单次混币请求的入池金额上限：**≤ 10,000 币**
- 整个池子内总余额上限：**≤ 100,000 币**
- 这两个规则写在 NeoVault Marble 的代码中，并通过 MarbleRun attestation 对外证明

#### 4.2.4 用户请求与 requestHash + TEE 签名

用户通过 CLI 或 API 发送混币请求（链下）：

**请求示例（链下 JSON）：**

```json
{
  "version": 1,
  "user_address": "NUser...",
  "input_txs": [
    "0xTxHash1",
    "0xTxHash2"
  ],
  "targets": [
    {"address": "NTarget1...", "amount": "5"},
    {"address": "NTarget2...", "amount": "5"}
  ],
  "mix_option": 3600000,
  "timestamp": 1730000000
}
```

NeoVault Marble 在 TEE 内：

1. 使用确定性序列化（例如 canonical JSON / protobuf）得到 `requestBytes`
2. 计算 `requestHash = Hash256(requestBytes)`
3. 使用 TEE 内 TEE 密钥对 `requestHash` 签名：`sig_TEE = Sign_TEE(requestHash)`

返回给用户：

```json
{
  "request": { "...原始字段..." },
  "request_hash": "0x...",
  "tee_signature": "0x..."
}
```

用户需要**妥善保存这三个字段**，以便将来发起 dispute 使用。

#### 4.2.5 正常混币流程（无 dispute）

**阶段 1：入池**

1. 用户本地使用自己的私钥，构造并签名一组转账交易，将资金从用户地址分散转入多个池地址（`Addr_pool*`）
2. 用户可以自行广播，也可以把已签名但未广播的交易交给 NeoVault 服务来分时广播
3. NeoVault 记录这些入池交易与 `requestHash` 的关联

**阶段 2：池内混淆**

- NeoVault TEE 在内部：
  - 在池地址之间调拨资金（噪声交易 / 拆分合并）
  - 控制时间窗口和转账图的形状，以提高链上分析难度（在账户模型下只能提高难度，不能保证强匿名）

**阶段 3：出池**

- 在约定的混币时间窗口内或之后，NeoVault 从池地址向用户目标地址发起一系列转账
- 这些转账在链上表现为：`Addr_poolX -> NTargetY` 等普通转账

**阶段 4：结束**

- 用户在本地只关心：
  - 自己目标地址收到预期金额
  - 不主动发起 dispute，即默认接受结果

#### 4.2.6 超时与争议（Dispute）流程

> 关键规则：**一旦进入 dispute 流程，就不再追求隐私，只追求可审计性与赔付能力。**

NeoVault 合约内部维护的 `RequestRecord`（逻辑结构示例）：

```csharp
struct RequestRecord
{
    ByteString RequestHash;
    UInt160    User;        // 用户地址
    ulong      Deadline;    // 超时时间戳 (请求时间 + mix_option + grace)
    bool       Fulfilled;   // 是否已提交履约证明
    bool       Disputed;    // 是否有发起争议
}
```

##### 步骤 1：用户发起争议

当用户认为在 deadline 后仍未收到满意结果，可以：

1. 将当时的 `request` 原始字段、`requestHash` 和 `teeSignature` 提交到 NeoVault 合约的 `Dispute` 方法：

```csharp
public static void Dispute(
    ByteString requestBytes,
    ByteString requestHash,
    ByteString teeSignature
)
```

2. 合约执行：
   - 检查 `Hash256(requestBytes) == requestHash`
   - 检查 `Verify(teeSignature, requestHash, TeePublicKey) == true`
   - 根据 requestBytes 中的 `timestamp` 和 `mix_option` 计算 deadline
   - 如果当前时间 > deadline（可含 grace period），则标记该请求为 `Disputed = true`

##### 步骤 2：NeoVault 提交履约证明

如果 NeoVault 确实已经完成该请求的混币：

1. NeoVault TEE 收集与该 `requestHash` 对应的"最终出池转账交易列表"：`txid_1, txid_2, ..., txid_n`
2. 计算：
   - `txidsHash = Hash256(txid_1 || txid_2 || ... || txid_n)`
   - `proofSig = Sign_TEE(requestHash || txidsHash)`
3. 调用合约：

```csharp
public static void ProveFulfilled(
    ByteString requestHash,
    ByteString txidsHash,
    ByteString proofSig
)
```

4. 合约验证 `Verify(proofSig, requestHash || txidsHash, TeePublicKey) == true`，记录 `Fulfilled = true`

##### 步骤 3：用户申请赔付

用户在 deadline + 宽限期之后，如果已发起 Dispute 但 NeoVault 未 ProveFulfilled：

```csharp
public static void ClaimCompensation(ByteString requestHash)
```

合约检查：`Disputed == true` && `Fulfilled == false` && 当前时间 > deadline + gracePeriod

若条件满足，从合约中的 **保证金池** 为用户支付约定金额作为赔偿。

#### 4.2.7 隐私与信任的权衡

- **正常路径**：用户只看见链上入池/出池交易，不知道中间路径；NeoVault 通过时间混淆、金额拆分和噪声交易提升分析难度
- **争议路径**：用户把原请求字段公开到链上，NeoVault 把交易列表哈希绑定到请求上，隐私被牺牲换取可审计性 + 赔付权
- **信任模型**：用户信任 TEE/MarbleRun 确保 NeoVault Marble 二进制和签名密钥未被篡改

**混币时长选项**: 30分钟 / 1小时 / 24小时 / 7天

**智能合约事件**:
```csharp
event ServiceRegistered(serviceId, teePubKey)
event BondDeposited(serviceId, amount, totalBond)
event DisputeOpened(requestHash, user, deadline)
event FulfillmentProved(requestHash, txidsHash)
event CompensationClaimed(requestHash, user, amount)
event BondSlashed(serviceId, slashedAmount, remainingBond)
```

**API 端点**:

| 端点 | 方法 | 说明 |
|------|------|------|
| `/neovault/info` | GET | 混币服务与池状态（额度、上限、负载） |
| `/neovault/request` | POST | 发起混币请求（返回 requestHash + TEE 签名） |
| `/neovault/status/{requestId}` | GET | 查询请求链下处理状态 |
| `/neovault/requests` | GET | 分页查看近期请求摘要 |

**合规性配置**:
- 单次混币请求入池金额上限：≤ 10,000 币
- 整体池余额上限：≤ 100,000 币
- 通过 MarbleRun attestation 对外证明规则未被更改

### 4.3 NeoFeeds 服务 (数据聚合)

**功能**: 聚合多源价格数据

**工作流程**:
```
1. 从多个数据源获取价格
         │
         ▼
2. 过滤异常值
         │
         ▼
3. 计算中位数/加权平均
         │
         ▼
4. 签名并存储
         │
         ▼
5. 提供给智能合约使用
```

**支持的价格对**:
- BTC/USD, ETH/USD, NEO/USD, GAS/USD

**API 端点**:
```
GET /neofeeds/prices
GET /neofeeds/prices/{pair}
GET /neofeeds/sources
```

### 4.4 NeoFlow 服务 (自动化)

**功能**: 定时任务和条件触发

**触发器类型**:
- `cron`: 定时执行 (如 "0 * * * *")
- `condition`: 条件触发 (如价格阈值)
- `event`: 事件触发 (如链上事件)

**API 端点**:
```
GET /neoflow/triggers
POST /neoflow/triggers
GET /neoflow/triggers/{id}
DELETE /neoflow/triggers/{id}
POST /neoflow/triggers/{id}/enable
POST /neoflow/triggers/{id}/disable
```

### 4.5 NeoCompute 服务 (机密计算) [规划中]

**状态**: 规划中 (Planned) - 当前为占位服务

**功能**: 在 TEE 内执行用户代码

**规划特性**:
- 支持 JS/Lua/WASM 脚本运行时
- 安全注入用户密钥
- 签名验证执行结果
- Gas 计量和资源限制

**计划 API 端点**:
```
POST /neocompute/execute
GET /neocompute/jobs/{id}
GET /neocompute/jobs
```

### 4.6 AccountPool 服务 (内部服务)

**状态**: 已实现 - 内部服务，不对外暴露

**功能**: 集中管理共享账户池，提供 HD 密钥派生和交易签名服务

**设计原则**:
- **私钥隔离**: 所有池账户私钥仅存在于 AccountPool 服务内部，永不离开此 TEE
- **服务调用**: 其他服务（如 NeoVault）通过 HTTP API 请求账户和签名
- **锁定机制**: 账户被服务锁定后，只有该服务可请求签名
- **确定性派生**: 使用 HKDF 从主密钥派生账户密钥，确保升级安全

**核心功能**:

| 功能 | 说明 |
|------|------|
| 账户生成 | HD 派生新账户，存储元数据到数据库 |
| 账户请求 | 服务请求并锁定指定数量的账户 |
| 账户释放 | 服务完成后释放账户回池 |
| 交易签名 | 为锁定账户签名交易哈希 |
| 余额更新 | 更新账户余额信息 |
| 账户轮换 | 每日轮换 10% 的账户 (24小时最小使用期) |

**API 端点** (内部):

| 端点 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/info` | GET | 服务状态信息 |
| `/request` | POST | 请求并锁定账户 |
| `/release` | POST | 释放账户 |
| `/sign` | POST | 签名单个交易 |
| `/batch-sign` | POST | 批量签名交易 |
| `/balance` | POST | 更新账户余额 |

**请求/响应示例**:

```json
// POST /request
{
  "service_id": "neovault",
  "count": 3,
  "purpose": "neovault-deposit"
}

// Response
{
  "accounts": [
    {"id": "acc-001", "address": "NXxx...", "balance": 0},
    {"id": "acc-002", "address": "NYyy...", "balance": 0},
    {"id": "acc-003", "address": "NZzz...", "balance": 0}
  ],
  "lock_id": "lock-123"
}

// POST /sign
{
  "service_id": "neovault",
  "account_id": "acc-001",
  "tx_hash": "0x..."
}

// Response
{
  "account_id": "acc-001",
  "signature": "0x...",
  "public_key": "0x..."
}
```

**使用此服务的其他服务**:
- **NeoVault**: 请求池账户进行混币操作

---

## 5. 安全模型

### 5.1 信任边界

```
┌─────────────────────────────────────────────────────────────┐
│                    不可信区域                                 │
│  • 网络流量                                                  │
│  • 操作系统                                                  │
│  • 云服务提供商                                               │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ 远程证明
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                    可信区域 (MarbleRun TEE)                    │
│  • 应用代码                                                  │
│  • 密钥材料                                                  │
│  • 敏感数据处理                                               │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 密钥管理

```
MarbleRun Coordinator
         │
         │ 证明验证后注入
         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  Service Marbles (各服务独立密钥)                                         │
│                                                                         │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐          │
│  │   VRF Marble    │  │ AccountPool     │  │ NeoFeeds       │          │
│  │                 │  │ Marble (内部)    │  │ Marble          │          │
│  │ VRF_PRIVATE_KEY │  │                 │  │                 │          │
│  │ (直接使用)       │  │ POOL_MASTER_KEY │  │ DATAFEEDS_KEY   │          │
│  └─────────────────┘  │       │         │  │ (签名密钥)       │          │
│                       │       ▼         │  └─────────────────┘          │
│                       │ HKDF 派生:       │                               │
│                       │ ┌─────────────┐ │                               │
│                       │ │Pool Account │ │                               │
│                       │ │Private Keys │ │  ← 私钥永不离开 AccountPool     │
│                       │ └─────────────┘ │                               │
│                       └────────┬────────┘                               │
│                                │                                        │
│                                │ HTTP API (签名请求)                     │
│                                ▼                                        │
│                       ┌─────────────────┐                               │
│                       │  NeoVault Marble   │                               │
│                       │                 │                               │
│                       │ MIXER_MASTER_KEY│  ← 用于请求哈希签名              │
│                       │ (不含池账户私钥)  │                               │
│                       └─────────────────┘                               │
└─────────────────────────────────────────────────────────────────────────┘
```

### 5.3 网络安全

- **外部通信**: HTTPS (TLS 1.2+)
- **服务间通信**: mTLS (TLS 1.3, 证书由 Coordinator 签发)
- **数据库通信**: TLS + API Key 认证

### 5.4 账户模型下的 NeoVault 隐私边界

对于 NeoVault 服务，特别增加如下说明：

- Neo N3 使用 **账户模型**，不是 UTXO 模型
- 池地址在脚本类型上与普通单签地址相同，但：
  - 其行为模式（资金集中来自/流向某些类型地址）仍可能被链上分析工具聚类分析
- NeoVault 在账户模型下提供的是：
  - **交易路径混淆 / 分时广播 / 金额拆分 / 噪声注入带来的"分析难度提升"**
  - 而不是在强对手模型下的形式化强匿名性
- 一旦用户发起 dispute：
  - 该笔请求的隐私被放弃用于公共审计与经济赔付
  - 这是用户主动在"隐私 vs 赔偿"间做的选择

---

## 6. 数据流程

### 6.0 核心架构：MarbleRun + EGo + Supabase + Neo N3 区块链

本平台基于四大核心技术构建完整的可信计算数据流：

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│              完整架构：MarbleRun + EGo + Supabase + Neo N3 Blockchain                 │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                     │
│  ┌────────────────────────────────────────────────────────────────────────────┐     │
│  │                      MarbleRun Coordinator (信任根)                         │     │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐            │     │
│  │  │  Manifest  │  │  Secrets   │  │Attestation │  │    PKI     │            │     │
│  │  │   验证      │  │   管理     │  │    引擎     │  │   证书      │            │     │
│  │  └────────────┘  └────────────┘  └────────────┘  └────────────┘            │     │
│  └──────────────────────────────┬─────────────────────────────────────────────┘     │
│                                 │ 远程证明 + 密钥注入                                  │
│                                 ▼                                                   │
│  ┌────────────────────────────────────────────────────────────────────────────┐     │
│  │                          EGo TEE 层 (可信执行)                           │     │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐            │     │
│  │  │  Gateway   │  │    VRF     │  │   NeoVault    │  │ NeoFeeds  │  ...       │     │
│  │  │  Marble    │  │  Marble    │  │  Marble    │  │  Marble    │            │     │
│  │  │ (EGo+MarbleRun)  │  │ (EGo+MarbleRun)  │  │ (EGo+MarbleRun)  │  │ (EGo+MarbleRun)  │            │     │
│  │  └──────┬─────┘  └──────┬─────┘  └──────┬─────┘  └──────┬─────┘            │     │
│  │         │               │               │               │                  │     │
│  │         └───────────────┴───────┬───────┴───────────────┘                  │     │
│  │                                 │                                          │     │
│  └─────────────────────────────────┼──────────────────────────────────────────┘     │
│                    ┌───────────────┼───────────────┐                                │
│                    │               │               │                                │
│                    ▼               ▼               ▼                                │
│  ┌─────────────────────┐  ┌─────────────────┐  ┌──────────────────────────────┐     │
│  │   Supabase (持久化)  │  │  Neo N3 RPC     │  │     Neo N3 智能合约层          │     │
│  │  ┌────────────────┐ │  │  (链交互)        │  │  ┌────────────────────────┐  │     │
│  │  │  PostgreSQL    │ │  │  • 事件监听      │  │  │  ServiceLayerGateway   │  │     │
│  │  │  + RLS 安全     │ │  │  • 交易广播      │  │  │  • 请求路由 • 费用管理    │  │     │
│  │  │                │ │  │  • 状态查询      │  │  │  • TEE 账户 • 回调执行    │  │    │
│  │  │  • users       │ │  └─────────────────┘  │  └────────────────────────┘  │     │
│  │  │  • api_keys    │ │                       │  ┌────────────────────────┐  │     │
│  │  │  • vrf_requests│ │                       │  │     VRFService         │  │     │
│  │  │  • neovault_*     │ │                       │  │  • 随机数请求/验证       │  │     │
│  │  │  • price_feeds │ │                       │  └────────────────────────┘  │     │
│  │  │  • gasbank_*   │ │                       │  ┌────────────────────────┐  │     │
│  │  │  • neoflow_*│ │                       │  │    NeoVaultService        │  │     │
│  │  └────────────────┘ │                       │  │  • 混币/争议/赔付        │  │     │
│  └─────────────────────┘                       │  └────────────────────────┘  │     │
│                                                │  ┌────────────────────────┐  │     │
│                                                │  │   NeoFeedsService     │  │     │
│                                                │  │  • 价格推送 • 数据验证    │  │     │
│                                                │  └────────────────────────┘  │     │
│                                                │  ┌────────────────────────┐  │     │
│                                                │  │  NeoFlowService     │  │     │
│                                                │  │  • 触发器 • 条件执行     │  │     │
│                                                │  └────────────────────────┘  │     │
│                                                └──────────────────────────────┘     │
│                                                                                     │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

**核心组件职责：**

| 组件 | 技术 | 职责 |
|------|------|------|
| **MarbleRun Coordinator** | MarbleRun v1.7+ | 管理 manifest、验证远程证明、注入密钥和 TLS 证书 |
| **EGo TEE** | EGo + MarbleRun/EGo | 在隔离环境中运行 Go 服务，保护密钥和敏感计算 |
| **Supabase** | PostgreSQL + RLS | 持久化所有业务数据，通过 RLS 实现行级安全 |
| **Neo N3 智能合约** | Neo N3 区块链 | 链上请求管理、费用结算、回调执行、状态验证 |

**智能合约架构：**

| 合约 | 功能 | 交互模式 |
|------|------|----------|
| **ServiceLayerGateway** | 统一入口、请求路由、费用管理、TEE 账户管理 | 用户 → Gateway → Service |
| **VRFService** | VRF 请求存储、随机数验证、结果回调 | 请求-响应 |
| **NeoVaultService** | 混币请求、争议处理、保证金管理 | 请求-响应 + 争议 |
| **NeoFeedsService** | 价格数据存储、数据源管理、新鲜度检查 | 推送模式 |
| **NeoFlowService** | 触发器注册、条件检查、执行记录 | 触发器模式 |

### 6.0.1 Marble 启动与密钥注入流程

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         Marble 启动流程 (以 VRF 服务为例)                          │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│   VRF Marble (EGo)                        MarbleRun Coordinator                 │
│        │                                           │                            │
│        │  1. 启动 EGo TEE                       │                            │
│        │  2. 生成 MarbleRun Quote (远程证明)               │                            │
│        │ ─────────────────────────────────────────►│                            │
│        │                                           │                            │
│        │                                   3. 验证 Quote:                        │
│        │                                      • MRENCLAVE 匹配 manifest?         │
│        │                                      • MRSIGNER 匹配?                   │
│        │                                      • ProductID = 2 (VRF)?            │
│        │                                      • SecurityVersion >= 1?           │
│        │                                           │                            │
│        │  4. 注入密钥和证书:                         │                            │
│        │     • VRF_PRIVATE_KEY                     │                            │
│        │     • MARBLE_CERT (TLS 证书)               │                            │
│        │     • MARBLE_KEY (TLS 私钥)                │                            │
│        │     • SUPABASE_URL                        │                            │
│        │     • SUPABASE_SERVICE_KEY                │                            │
│        │ ◄─────────────────────────────────────────│                            │
│        │                                           │                            │
│        │  5. 初始化 Supabase 客户端                  │                            │
│        │  6. 开始处理请求                            │                            │
│        │                                           │                            │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 6.0.2 完整数据流：TEE ↔ Supabase ↔ Neo N3 区块链

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                    完整数据流 (链上请求 → TEE 处理 → 链上回调)                               │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                         │
│   用户/DApp           Neo N3 区块链                TEE (EGo TEE)          Supabase    │
│      │                    │                              │                      │       │
│      │  1. 调用用户合约     │                              │                      │       │
│      │ ──────────────────►│                              │                      │       │
│      │                    │                              │                      │       │
│      │              ┌─────┴─────┐                        │                      │       │
│      │              │UserContract│                       │                      │       │
│      │              │RequestVRF()│                       │                      │       │
│      │              └─────┬─────┘                        │                      │       │
│      │                    │ 2. RequestService()          │                      │       │
│      │              ┌─────┴─────┐                        │                      │       │
│      │              │  Gateway  │                        │                      │       │
│      │              │  Contract │                        │                      │       │
│      │              └─────┬─────┘                        │                      │       │
│      │                    │ 3. OnRequest()               │                      │       │
│      │              ┌─────┴─────┐                        │                      │       │
│      │              │VRFService │                        │                      │       │
│      │              │ Contract  │                        │                      │       │
│      │              └─────┬─────┘                        │                      │       │
│      │                    │ 4. Emit VRFRequest Event     │                      │       │
│      │                    │ ════════════════════════════►│                      │       │
│      │                    │                              │                      │       │
│      │                    │                              │  5. 解析事件数据       │       │
│      │                    │                              │  6. 记录请求到 DB      │       │
│      │                    │                              │ ────────────────────►│       │
│      │                    │                              │                      │       │
│      │                    │                              │  7. VRF 计算          │       │
│      │                    │                              │  (使用注入的私钥)      │       │
│      │                    │                              │                      │       │
│      │                    │                              │  8. 更新状态到 DB      │       │
│      │                    │                              │ ────────────────────►│       │
│      │                    │                              │                      │       │
│      │                    │  9. FulfillRequest()         │                      │       │
│      │                    │ ◄════════════════════════════│                      │       │
│      │              ┌─────┴─────┐                        │                      │       │
│      │              │  Gateway  │                        │                      │       │
│      │              │ 验证TEE签名│                        │                      │       │
│      │              └─────┬─────┘                        │                      │       │
│      │                    │ 10. Callback()               │                      │       │
│      │              ┌─────┴─────┐                        │                      │       │
│      │              │UserContract│                       │                      │       │
│      │              │  接收随机数 │                        │                      │       │
│      │              └─────┬─────┘                        │                      │       │
│      │  11. 交易确认       │                              │                      │       │
│      │ ◄──────────────────│                              │                      │       │
│      │                    │                              │                      │       │
└─────────────────────────────────────────────────────────────────────────────────────────┘
```

**数据流说明：**

| 步骤 | 组件 | 操作 | 数据存储 |
|------|------|------|----------|
| 1-4 | 链上合约 | 用户发起请求 → Gateway 路由 → Service 合约发出事件 | Neo N3 区块链 |
| 5-6 | TEE | 监听链上事件，解析请求数据，记录到数据库 | Supabase |
| 7 | TEE | 使用 Coordinator 注入的私钥进行 VRF 计算 | 内存 (TEE) |
| 8 | TEE | 更新请求状态和结果到数据库 | Supabase |
| 9-11 | TEE → 链上 | TEE 签名结果，调用 Gateway 回调，通知用户合约 | Neo N3 区块链 |

### 6.0.3 服务模式概述

Service Layer 支持三种不同的服务模式：

| 模式 | 服务 | 说明 |
|------|------|------|
| **请求-响应** | VRF, NeoVault, NeoCompute | 用户发起请求 → TEE 处理 → 回调 |
| **推送 (自动更新)** | NeoFeeds | TEE 定期更新链上数据，无需用户请求 |
| **触发器** | NeoFlow | 用户注册触发器 → TEE 监控条件 → 周期性回调 |

### 6.1 模式一：请求-响应 (VRF, NeoVault, NeoCompute)

从用户到回调的完整请求流程：

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           请求流程 (步骤 1-4)                                  │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────┐    ┌───────────────┐    ┌─────────────────────┐    ┌────────────┐  │
│  │ 用户  │───►│   用户合约     │───►│ ServiceLayerGateway │───►│   服务      │  │
│  └──────┘    │               │    │     (网关合约)       │    │   合约      │  │
│     1        │  RequestVRF() │    │  RequestService()   │    │ OnRequest()│  │
│              └───────────────┘    └─────────────────────┘    └─────┬──────┘  │
│                     2                       3                    4 │         │
│                                                                    ▼         │
│                                                              ┌────────────┐  │
│                                                              │   事件      │  │
│                                                              │  (链上)     │  │
│                                                              └─────┬──────┘  │
└────────────────────────────────────────────────────────────────────┼─────────┘
                                                                     │
┌────────────────────────────────────────────────────────────────────┼────────┐
│                         服务层 (链下 TEE)                            │        │
├────────────────────────────────────────────────────────────────────┼────────┤
│                                                                    ▼        │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                    Service Layer (TEE TEE)                      │    │
│  │  5. 监听区块链事件                                                     │   │
│  │  6. 处理请求 (VRF 计算 / 混币执行 / 机密计算)                            │    │
│  │  7. 使用 TEE 私钥签名结果                                              │    │
│  └──────────────────────────────────┬──────────────────────────────────┘    │
│                                     │                                       │
└─────────────────────────────────────┼───────────────────────────────────────┘
                                      │
┌─────────────────────────────────────┼───────────────────────────────────────┐
│                           回调流程 (步骤 8-11)                                │
├─────────────────────────────────────┼───────────────────────────────────────┤
│                                     ▼                                       │
│  ┌──────┐    ┌───────────────┐    ┌─────────────────────┐    ┌────────────┐ │
│  │ 用户  │◄───│   用户合约     │◄───│ ServiceLayerGateway │◄───│   服务      │ │
│  └──────┘    │               │    │     (网关合约)       │    │   合约      │ │
│    11        │   Callback()  │    │  FulfillRequest()   │    │ OnFulfill()│ │
│              └───────────────┘    └─────────────────────┘    └────────────┘ │
│                    10                       9                      8        │
└─────────────────────────────────────────────────────────────────────────────┘
```

**步骤详解:**

| 步骤 | 组件 | 方法 | 说明 |
|------|------|------|------|
| 1 | 用户 | - | 用户发起交易调用其合约 |
| 2 | 用户合约 | `RequestVRF()` | 构建请求载荷，调用网关 |
| 3 | ServiceLayerGateway | `RequestService()` | 验证请求，扣除费用，路由到服务合约 |
| 4 | 服务合约 | `OnRequest()` | 存储请求数据，发出服务特定事件 |
| 5 | 服务层 (TEE) | - | 监听区块链事件 |
| 6 | 服务层 (TEE) | - | 链下处理请求 (VRF 计算 / 混币执行) |
| 7 | 服务层 (TEE) | - | 使用 TEE 私钥签名结果 |
| 8 | 服务合约 | `OnFulfill()` | 接收完成通知，清理请求数据 |
| 9 | ServiceLayerGateway | `FulfillRequest()` | 验证 TEE 签名，更新请求状态 |
| 10 | 用户合约 | `Callback()` | 接收结果，更新应用状态 |
| 11 | 用户 | - | 交易在区块链上确认 |

### 6.2 模式二：推送/自动更新 (NeoFeeds)

NeoFeeds 服务自动更新链上价格数据，无需用户请求：

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    服务层 (TEE) - 持续循环                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  1. 从多个数据源获取价格 (Binance, Coinbase 等)                         │    │
│  │  2. 聚合并验证数据 (中位数, 异常值过滤)                                  │    │
│  │  3. 使用 TEE 密钥签名聚合价格                                          │    │
│  │  4. 定期提交到 NeoFeedsService 合约                                  │    │
│  └──────────────────────────────────┬──────────────────────────────────┘    │
└─────────────────────────────────────┼───────────────────────────────────────┘
                                      │ UpdatePrice()
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      NeoFeedsService 合约                                   │
│  • 存储最新价格 (BTC/USD, ETH/USD, NEO/USD, GAS/USD 等)                       │
│  • 验证 TEE 签名                                                             │
│  • 发出 PriceUpdated 事件                                                    │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │ getLatestPrice()
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         用户合约 (只读)                                       │
│  • DeFi 协议直接读取价格                                                       │
│  • 无需回调 - 直接查询当前价格                                                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.3 模式三：触发器 (NeoFlow)

用户注册触发器，TEE 监控条件并周期性调用回调：

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                       触发器注册 (一次性)                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌──────┐    ┌───────────────┐    ┌─────────────────────┐    ┌────────────┐ │
│  │ 用户  │───►│   用户合约     │───►│ ServiceLayerGateway │───►│ NeoFlow │ │
│  └──────┘    │               │    │  RequestService()   │    │  Service   │ │
│              │ RegisterTrigger│   └─────────────────────┘    │ OnRequest()│ │
│              └───────────────┘                               └─────┬──────┘ │
│                                                                    │        │
│  触发器类型:                                                         ▼        │
│  • 时间触发: "每周五 00:00 UTC"                              ┌────────────┐   │
│  • 价格触发: "当 BTC > $100,000"                            │    触发器    │  │
│  • 事件触发: "当合约 X 发出事件 Y"                            │  已注册      │  │
│                                                            └────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
┌─────────────────────────────────────┼───────────────────────────────────────┐
│                            服务层 (TEE) - 持续监控                            │
├─────────────────────────────────────┼───────────────────────────────────────┤
│                                     ▼                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  循环: 检查所有已注册的触发器                                           │    │
│  │  • 时间触发: 比较当前时间                                              │    │
│  │  • 价格触发: 检查 NeoFeeds 价格                                       │    │
│  │  • 事件触发: 监控区块链事件                                             │    │
│  │  当条件满足 → 执行回调                                                 │    │
│  └──────────────────────────────────┬──────────────────────────────────┘    │
└─────────────────────────────────────┼───────────────────────────────────────┘
                                      │ 条件满足
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         回调执行 (周期性)                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌──────┐    ┌───────────────┐    ┌─────────────────────┐    ┌────────────┐ │
│  │ 用户  │◄───│   用户合约     │◄───│ ServiceLayerGateway │◄───│ NeoFlow │ │
│  └──────┘    │   Callback()  │    │  FulfillRequest()   │    │  Service   │ │
│              │ (如: rebase)  │    └─────────────────────┘    └────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

**NeoFlow 触发器示例:**

| 触发器类型 | 示例 | 用例 |
|------------|------|------|
| 时间触发 | `cron: "0 0 * * FRI"` | 每周代币分发 |
| 价格触发 | `price: BTC > 100000` | 价格达标自动卖出 |
| 阈值触发 | `balance < 10 GAS` | 自动充值 Gas 银行 |
| 事件触发 | `event: LiquidityAdded` | 响应链上事件 |

### 6.4 API 请求处理流程

```
客户端                Gateway              Service              Database
   │                    │                    │                    │
   │   1. HTTPS 请求     │                    │                    │
   │ ─────────────────► │                    │                    │
   │                    │                    │                    │
   │                    │   2. 验证 JWT       │                    │
   │                    │ ──────────────────►│                    │
   │                    │                    │                    │
   │                    │   3. mTLS 转发      │                    │
   │                    │ ──────────────────►│                    │
   │                    │                    │                    │
   │                    │                    │   4. 查询/存储      │
   │                    │                    │ ──────────────────►│
   │                    │                    │                    │
   │                    │                    │   5. 返回数据       │
   │                    │                    │ ◄──────────────────│
   │                    │                    │                    │
   │                    │   6. 签名响应       │                    │
   │                    │ ◄──────────────────│                    │
   │                    │                    │                    │
   │   7. 返回结果       │                    │                    │
   │ ◄───────────────── │                    │                    │
```

### 6.5 证明流程

```
Marble                          Coordinator
   │                                 │
   │   1. 生成 MarbleRun Quote              │
   │ ───────────────────────────────►│
   │                                 │
   │                                 │  2. 验证 Quote
   │                                 │     - 检查 MRENCLAVE
   │                                 │     - 检查 MRSIGNER
   │                                 │     - 检查 ProductID
   │                                 │     - 检查 SecurityVersion
   │                                 │
   │    3. 注入密钥和证书              │
   │ ◄───────────────────────────────│
   │                                 │
   │   4. 开始服务                    │
   │                                 │
```

---

## 7. 部署指南

### 7.1 环境要求

**硬件要求**:
- MarbleRun/EGo 支持的 CPU (生产环境)
- 最少 8GB RAM
- 50GB SSD

**软件要求**:
- Docker 20.10+
- Docker Compose 2.0+
- Go 1.21+ (开发)

### 7.2 模拟模式部署

```bash
# 设置模拟模式
export OE_SIMULATION=1

# 启动所有服务
cd docker
docker compose up -d

# 查看日志
docker compose logs -f gateway
```

### 7.3 生产模式部署

```bash
# 确保 MarbleRun 驱动已安装
ls /dev/tee

# 设置生产模式
export OE_SIMULATION=0

# 设置 manifest
marblerun manifest set manifests/manifest.json

# 启动服务
docker compose up -d
```

### 7.4 环境变量

```bash
# .env 文件
SUPABASE_URL=https://xxx.supabase.co
SUPABASE_SERVICE_KEY=eyJxxx...
NEO_RPC_URL=https://mainnet.neo.coz.io
OE_SIMULATION=1
```

### 7.5 健康检查

```bash
# 检查 Gateway 健康状态
curl http://localhost:8080/health

# 检查证明状态
curl http://localhost:8080/attestation
```

---

## 8. API 参考

### 8.1 认证

所有 API 请求需要在 Header 中包含:
```
Authorization: Bearer <JWT_TOKEN>
```

### 8.2 通用响应格式

**成功响应**:
```json
{
    "success": true,
    "data": {...},
    "timestamp": "2024-12-06T12:00:00Z"
}
```

**错误响应**:
```json
{
    "success": false,
    "error": "error message",
    "timestamp": "2024-12-06T12:00:00Z"
}
```

### 8.3 端点汇总

| 服务 | 端点 | 方法 | 说明 |
|------|------|------|------|
| Gateway | `/health` | GET | 健康检查 |
| Gateway | `/attestation` | GET | 证明状态 |
| Gateway | `/api/v1/auth/register` | POST | 用户注册 |
| Gateway | `/api/v1/auth/login` | POST | 用户登录 |
| VRF | `/neorand/random` | POST | 生成随机数 |
| VRF | `/neorand/verify` | POST | 验证随机数 |
| NeoVault | `/neovault/info` | GET | 混币服务与池状态 |
| NeoVault | `/neovault/request` | POST | 发起混币请求（返回 requestHash + TEE 签名） |
| NeoVault | `/neovault/status/{requestId}` | GET | 查询请求链下处理状态 |
| NeoVault | `/neovault/requests` | GET | 分页查看近期请求摘要 |
| NeoFeeds | `/neofeeds/prices` | GET | 获取价格 |
| NeoFeeds | `/neofeeds/prices/{pair}` | GET | 获取指定交易对价格 |
| NeoFeeds | `/neofeeds/sources` | GET | 获取数据源列表 |
| NeoFlow | `/neoflow/triggers` | GET/POST | 触发器管理 |
| NeoFlow | `/neoflow/triggers/{id}` | GET/DELETE | 查询/删除触发器 |
| NeoCompute | `/neocompute/execute` | POST | 执行机密计算任务 [规划中] |
| NeoCompute | `/neocompute/jobs/{id}` | GET | 查询任务状态 [规划中] |

**内部服务 API** (仅限服务间调用):

| 服务 | 端点 | 方法 | 说明 |
|------|------|------|------|
| AccountPool | `/request` | POST | 请求并锁定账户 |
| AccountPool | `/release` | POST | 释放账户 |
| AccountPool | `/sign` | POST | 签名交易 |
| AccountPool | `/batch-sign` | POST | 批量签名 |
| AccountPool | `/balance` | POST | 更新余额 |

---

## 9. NeoVault 安全与争议处理模型

本节可作为白皮书/安全审计章节的内容引用。

### 9.1 目标

- 在 Neo N3 账户模型下提供一套 **隐私增强的链下混币服务**
- 保证：
  - 用户与 NeoVault 之间的请求具有 **可验证的承诺**（requestHash + TEE 签名）
  - 当 NeoVault 未在约定时间履约时，用户可以通过链上合约发起争议并获得赔偿
  - 一旦进入争议流程，隐私让位于可审计性

### 9.2 信任假设

- **信任** MarbleRun/EGo / TEE 硬件 + MarbleRun 协调器能够：
  - 保证 NeoVault Marble 二进制以及内部密钥不会被恶意方篡改或泄露
- **不信任**：
  - 外部操作系统、云平台、网络
- **针对 NeoVault**：
  - 默认不信任服务运营方的"口头承诺"，而是通过：
    - TEE 证明 + 合约验证 TEE 签名 + 经济保证金
    来强制其履约

### 9.3 核心机制

1. **请求承诺 (Commitment)**
   每次混币请求通过 `requestHash = Hash(requestBytes)` 和 `sig_TEE = Sign_TEE(requestHash)` 被 TEE 承诺。
   用户持有 `(requestBytes, requestHash, sig_TEE)`，可在未来于链上证明"NeoVault TEE 确实接受过这样一个请求"。

2. **账户池混币 (通过 AccountPool 服务)**
   - 使用一组由 **AccountPool 内部服务** 管理的 HD 池地址收款和分发
   - 池账户私钥仅存在于 AccountPool 服务内，NeoVault 通过 HTTP API 请求签名
   - 按策略控制时间和金额分布，以提升链上分析成本

3. **额度与合规控制**
   NeoVault Marble 内部硬编码：
   - 单请求入池金额上限（≤ 10,000 币）
   - 整体池余额上限（≤ 100,000 币）
   外界可通过 attestation 验证该逻辑未被更改。

4. **争议与赔付**
   - 用户可在超时后，将原请求明文 + `requestHash` + `sig_TEE` 发往 NeoVault 合约
   - 合约验证通过后，记为 Disputed
   - NeoVault 可提交 TEE 对 `(requestHash || txidsHash)` 的签名作为履约证明
   - 若超时未提交履约证明，则合约从保证金池对用户进行赔付

5. **隐私与补偿的权衡**
   - 正常路径下，NeoVault 只在链上留下常规账户转账轨迹，不公开请求详细内容
   - 争议路径下，用户主动公开请求内容与目标地址，换取赔偿权
   - 这是用户在使用前需知晓的安全/隐私权衡

---

## 10. 服务升级安全性

### 10.1 设计原则

Neo Service Layer 的密钥管理设计确保 **TEE 升级不会影响任何业务密钥**。这是通过以下原则实现的：

1. **所有业务密钥来自 MarbleRun 注入**
   - 密钥通过 manifest 定义，由 Coordinator 在证明通过后注入
   - 密钥存储在 Coordinator 的 sealed state 中，而非 Marble 本地

2. **密钥派生不依赖 TEE 身份**
   - HKDF 派生上下文仅使用业务标识符（如 accountID）
   - 不使用 MRENCLAVE、MRSIGNER 或其他 TEE 测量值

3. **无本地 sealing key 依赖**
   - 业务数据不使用 TEE sealing key 加密
   - 所有持久化数据存储在 Supabase，使用 MarbleRun 注入的密钥加密

### 10.2 密钥架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          密钥层次结构（升级安全）                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   MarbleRun Coordinator (信任根)                                             │
│   ┌────────────────────────────────────────────────────────────────────┐    │
│   │  Manifest 定义的密钥 (升级后保持不变)                                  │    │
│   │  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  │    │
│   │  │ VRF_PRIVATE_KEY  │  │ MIXER_MASTER_KEY │  │ DATAFEEDS_KEY    │  │    │
│   │  │ (VRF 签名密钥)    │  │ (NeoVault 主密钥)    │  │ (数据签名密钥)     │  │    │
│   │  └────────┬─────────┘  └────────┬─────────┘  └──────────────────┘  │    │
│   └───────────┼─────────────────────┼──────────────────────────────────┘    │
│               │                     │                                       │
│               │ 证明通过后注入        │ 证明通过后注入                           │
│               ▼                     ▼                                       │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                     EGo TEE (可升级)                             │   │
│   │                                                                     │   │
│   │   NeoRand Service                    NeoVault Service                      │   │
│   │   ┌─────────────────┐           ┌─────────────────────────────────┐ │   │
│   │   │ privateKey      │           │ masterKey                       │ │   │
│   │   │ (直接使用)       │           │     │                           │ │   │
│   │   └─────────────────┘           │     │ HKDF 派生 (无 TEE ID)  │ │   │
│   │                                 │     ▼                           │ │   │
│   │                                 │ DeriveKey(masterKey,            │ │   │
│   │                                 │           accountID,            │ │   │
│   │                                 │           "neovault-account", 32)  │ │   │
│   │                                 │     │                           │ │   │
│   │                                 │     ▼                           │ │   │
│   │                                 │ Pool Account Keys               │ │   │
│   │                                 │ (确定性派生，升级后相同)            │ │   │
│   │                                 └─────────────────────────────────┘ │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 10.3 HKDF 密钥派生

所有派生密钥使用 HKDF-SHA256，**派生上下文不包含 TEE 身份**：

```go
// internal/crypto/crypto.go
func DeriveKey(masterKey []byte, salt []byte, info string, keyLen int) ([]byte, error) {
    // 输入:
    // - masterKey: 来自 MarbleRun 注入 (升级后不变)
    // - salt: 业务标识符如 accountID (与 TEE 无关)
    // - info: 用途字符串如 "neovault-account" (与 TEE 无关)
    //
    // 结果: 升级 TEE 后，相同输入产生相同输出
    hkdfReader := hkdf.New(sha256.New, masterKey, salt, []byte(info))
    key := make([]byte, keyLen)
    io.ReadFull(hkdfReader, key)
    return key, nil
}
```

**派生参数对比**：

| 参数 | 来源 | 包含 TEE ID? | 升级后变化? |
|------|------|-----------------|------------|
| masterKey | MarbleRun 注入 | ❌ 否 | ❌ 不变 |
| salt | 业务 ID (如 accountID) | ❌ 否 | ❌ 不变 |
| info | 用途字符串 | ❌ 否 | ❌ 不变 |

### 10.4 各服务密钥来源

| 服务 | 密钥 | 来源 | 派生方式 | 升级安全 |
|------|------|------|----------|---------|
| **VRF** | VRF_PRIVATE_KEY | Marble.Secret() | 直接使用 | ✅ |
| **AccountPool** | POOL_MASTER_KEY | Marble.Secret() | 直接使用 | ✅ |
| **AccountPool 池账户** | 池账户私钥 | DeriveKey() | HKDF(masterKey, accountID, "pool-account") | ✅ |
| **NeoVault** | MIXER_MASTER_KEY | Marble.Secret() | 直接使用 (用于请求签名) | ✅ |
| **NeoFeeds** | DATAFEEDS_SIGNING_KEY | Marble.Secret() | 直接使用 | ✅ |
| **NeoFlow** | AUTOMATION_KEY | Marble.Secret() | 直接使用 | ✅ |
| **TLS** | MARBLE_CERT/KEY | Coordinator 签发 | 每次启动重新签发 | ✅ |

**注意**: NeoVault 服务不再直接持有池账户私钥，而是通过 HTTP API 请求 AccountPool 服务进行签名。

### 10.5 升级流程

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          安全升级流程                                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. 准备新版本                                                                │
│     ┌─────────────────────────────────────────────────────────────────┐     │
│     │  • 构建新的 EGo TEE 二进制                                    │     │
│     │  • 获取新的 MRENCLAVE 值                                          │     │
│     │  • 代码审计确保无 sealing key 或 TEE ID 依赖                    │     │
│     └─────────────────────────────────────────────────────────────────┘     │
│                                      │                                       │
│                                      ▼                                       │
│  2. 更新 Manifest                                                            │
│     ┌─────────────────────────────────────────────────────────────────┐     │
│     │  • 更新 Packages 中对应服务的 UniqueID (MRENCLAVE)                 │     │
│     │  • 可选: 增加 SecurityVersion                                    │      │
│     │  • 保持 Secrets 部分不变 (密钥定义相同)                             │      │
│     └─────────────────────────────────────────────────────────────────┘      │
│                                      │                                       │
│                                      ▼                                       │
│  3. 应用 Manifest 更新                                                       │
│     ┌─────────────────────────────────────────────────────────────────┐     │
│     │  marblerun manifest update manifests/manifest.json              │     │
│     │  • Coordinator 验证更新签名                                       │     │
│     │  • 新旧 Marble 共存期间服务不中断                                   │     │
│     └─────────────────────────────────────────────────────────────────┘     │
│                                      │                                      │
│                                      ▼                                      │
│  4. 滚动更新 Marbles                                                         │
│     ┌─────────────────────────────────────────────────────────────────┐     │
│     │  • 新 Marble 启动，生成新 MRENCLAVE 的 Quote                      │     │
│     │  • Coordinator 验证 Quote 匹配更新后的 Manifest                   │     │
│     │  • Coordinator 注入相同的密钥 (VRF_PRIVATE_KEY 等)                │     │
│     │  • 新 Marble 派生出相同的池账户密钥                                 │     │
│     │  • 服务继续运行，密钥完全相同                                       │     │
│     └─────────────────────────────────────────────────────────────────┘     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 10.6 验证清单

升级前应验证：

- [ ] 代码不使用 `enclave.GetSealKey()` 或类似 API
- [ ] HKDF 派生不包含 MRENCLAVE/MRSIGNER 作为输入
- [ ] 业务数据不使用 TEE sealing 加密
- [ ] manifest 中的 Secrets 定义与旧版本兼容
- [ ] 池账户等派生密钥可以从相同主密钥重建

### 10.7 不支持的操作

以下操作会导致密钥变化，**不应执行**：

| 操作 | 后果 | 严重性 |
|------|------|--------|
| 修改 manifest 中的 Secret 值 | 所有服务密钥变化 | 🔴 严重 |
| 在 HKDF 中添加 TEE ID | 派生密钥变化 | 🔴 严重 |
| 使用 sealing key 加密业务数据 | 升级后无法解密 | 🔴 严重 |
| 更改 HKDF info 字符串 | 派生密钥变化 | 🟡 中等 |

---

## 附录

### A. 测试状态

```
✅ internal/crypto      - 38 tests passed
✅ internal/database    - 22 tests passed
✅ internal/marble      - 18 tests passed
✅ internal/gasbank     - 7 tests passed
✅ services/neorand         - 6 tests passed
✅ services/neovault       - 3 tests passed
✅ services/accountpool - (内部服务，无独立测试)
✅ services/neofeeds   - 6 tests passed
✅ services/neoflow  - 6 tests passed
✅ services/neocompute - 5 tests passed
✅ test/integration     - 12 tests passed
```

### B. 版本信息

- **文档版本**: 3.2.0
- **项目版本**: 3.2.0
- **最后更新**: 2025-01-15
- **作者**: Neo Service Layer Team
- **变更记录**:
  - v3.2.0: 添加 AccountPool 内部服务，重构 NeoVault 使用 AccountPool 进行账户管理
  - v3.1.0: 添加服务升级安全性章节 (Section 10)
  - v3.0.0: 添加完整四层架构文档 (MarbleRun + EGo + Supabase + Neo N3)

### C. 参考链接

- [MarbleRun 文档](https://docs.edgeless.systems/marblerun)
- [EGo 文档](https://docs.edgeless.systems/ego)
- [Supabase 文档](https://supabase.com/docs)
- [Netlify 文档](https://docs.netlify.com)
- [Neo N3 文档](https://docs.neo.org)
