# Service Layer Neo N3 Smart Contracts

## 概述

Service Layer 智能合约系统为 Neo N3 区块链提供去中心化的预言机、VRF、隐私混币等服务。所有服务通过统一的 Gateway 合约进行管理和路由。

## 架构

### 服务模式概述

Service Layer 支持三种不同的服务模式：

| 模式 | 服务 | 说明 |
|------|------|------|
| **请求-响应** | Oracle, VRF, NeoVault, NeoCompute | 用户发起请求 → TEE 处理 → 回调 |
| **推送 (自动更新)** | NeoFeeds | TEE 定期更新链上数据，无需用户请求 |
| **触发器** | NeoFlow | 用户注册触发器 → TEE 监控条件 → 周期性回调 |

### 模式一：请求-响应流程图

从用户到回调的完整请求流程：

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           请求流程 (步骤 1-4)                                  │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────┐    ┌───────────────┐    ┌─────────────────────┐    ┌────────────┐ │
│  │ 用户 │───►│   用户合约     │───►│ ServiceLayerGateway │───►│   服务     │ │
│  └──────┘    │               │    │     (网关合约)       │    │   合约     │ │
│     1        │ RequestPrice()│    │  RequestService()   │    │ OnRequest()│ │
│              └───────────────┘    └─────────────────────┘    └─────┬──────┘ │
│                     2                       3                      4 │      │
│                                                                      ▼      │
│                                                              ┌────────────┐ │
│                                                              │   事件     │ │
│                                                              │  (链上)    │ │
│                                                              └─────┬──────┘ │
└────────────────────────────────────────────────────────────────────┼────────┘
                                                                     │
┌────────────────────────────────────────────────────────────────────┼────────┐
│                        服务层 (链下 TEE)                            │        │
├────────────────────────────────────────────────────────────────────┼────────┤
│                                                                    ▼        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    Service Layer (MarbleRun TEE)                       │   │
│  │  5. 监听区块链事件                                                    │   │
│  │  6. 处理请求 (HTTP 获取 / VRF 计算 / 混币执行)                        │   │
│  │  7. 使用 TEE 私钥签名结果                                             │   │
│  └──────────────────────────────────┬──────────────────────────────────┘   │
│                                     │                                       │
└─────────────────────────────────────┼───────────────────────────────────────┘
                                      │
┌─────────────────────────────────────┼───────────────────────────────────────┐
│                        回调流程 (步骤 8-11)                         │        │
├─────────────────────────────────────┼───────────────────────────────────────┤
│                                     ▼                                       │
│  ┌──────┐    ┌───────────────┐    ┌─────────────────────┐    ┌────────────┐│
│  │ 用户 │◄───│   用户合约     │◄───│ ServiceLayerGateway │◄───│   服务     ││
│  └──────┘    │               │    │     (网关合约)       │    │   合约     ││
│    11        │   Callback()  │    │  FulfillRequest()   │    │ OnFulfill()││
│              └───────────────┘    └─────────────────────┘    └────────────┘│
│                    10                       9                      8        │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 合约架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         用户合约                                  │
│                    (ExampleConsumer)                             │
│  • RequestPrice()      • RequestRandom()     • OnServiceCallback │
└─────────────────────────┬───────────────────────────────────────┘
                          │ RequestService()
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                   ServiceLayerGateway                            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐               │
│  │ 费用管理     │ │ TEE账户管理  │ │ 服务注册     │               │
│  └─────────────┘ └─────────────┘ └─────────────┘               │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐               │
│  │ 请求路由     │ │ 回调执行     │ │ 重放保护     │               │
│  └─────────────┘ └─────────────┘ └─────────────┘               │
└─────────────────────────┬───────────────────────────────────────┘
                          │ OnRequest()
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                      服务合约层                                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │ Oracle   │ │   VRF    │ │  NeoVault   │ │ NeoFeeds│          │
│  │ Service  │ │ Service  │ │ Service  │ │ Service  │          │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘          │
└───────┼────────────┼────────────┼────────────┼──────────────────┘
        │            │            │            │
        └────────────┴────────────┴────────────┘
                          │ Events (OracleRequest, VRFRequest, etc.)
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                    TEE (可信执行环境)                             │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐               │
│  │ 事件监听     │ │ 请求处理     │ │ 签名回调     │               │
│  └─────────────┘ └─────────────┘ └─────────────┘               │
└─────────────────────────┬───────────────────────────────────────┘
                          │ FulfillRequest()
                          ▼
                   ServiceLayerGateway
                          │ Callback
                          ▼
                       用户合约
```

## 请求流程详解

### 步骤说明

| 步骤 | 组件 | 方法 | 说明 |
|------|------|------|------|
| 1 | 用户 | - | 用户发起交易调用其合约 |
| 2 | 用户合约 | `RequestPrice()` | 构建请求载荷，调用网关 |
| 3 | ServiceLayerGateway | `RequestService()` | 验证请求，扣除费用，路由到服务合约 |
| 4 | 服务合约 | `OnRequest()` | 存储请求数据，发出服务特定事件 |
| 5 | 服务层 (TEE) | - | 监听区块链事件 |
| 6 | 服务层 (TEE) | - | 链下处理请求 (HTTP/VRF/Mix) |
| 7 | 服务层 (TEE) | - | 使用 TEE 私钥签名结果 |
| 8 | 服务合约 | `OnFulfill()` | 接收完成通知，清理请求数据 |
| 9 | ServiceLayerGateway | `FulfillRequest()` | 验证 TEE 签名，更新请求状态 |
| 10 | 用户合约 | `Callback()` | 接收结果，更新应用状态 |
| 11 | 用户 | - | 交易在区块链上确认 |

### 1. 用户发起请求

```
用户 → 用户合约.RequestPrice() → Gateway.RequestService("oracle", payload, "onCallback")
```

### 2. Gateway 处理请求

1. 验证合约未暂停
2. 检查服务已注册
3. 扣除用户费用
4. 生成请求 ID
5. 调用服务合约 `OnRequest()`
6. 发出 `ServiceRequest` 事件

### 3. 服务合约处理

1. 验证调用者是 Gateway
2. 解析请求参数
3. 存储请求数据
4. 发出服务特定事件 (如 `OracleRequest`)

### 4. TEE 处理

1. 监听服务合约事件
2. 执行链下处理 (HTTP 请求、VRF 计算等)
3. 签名结果
4. 调用 `Gateway.FulfillRequest()`

### 5. 回调执行

1. Gateway 验证 TEE 签名
2. 调用服务合约 `OnFulfill()`
3. 执行用户合约回调方法

### 模式二：推送/自动更新 (NeoFeeds)

NeoFeeds 服务自动更新链上价格数据，无需用户请求：

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    服务层 (TEE) - 持续循环                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│  1. 从多个数据源获取价格 (Binance, Coinbase 等)                               │
│  2. 聚合并验证数据 (中位数, 异常值过滤)                                       │
│  3. 使用 TEE 密钥签名聚合价格                                                 │
│  4. 定期提交到 NeoFeedsService 合约                                          │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │ UpdatePrice()
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      NeoFeedsService 合约                                   │
│  • 存储最新价格 (BTC/USD, ETH/USD, NEO/USD, GAS/USD 等)                     │
│  • 验证 TEE 签名                                                             │
│  • 发出 PriceUpdated 事件                                                    │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │ getLatestPrice()
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         用户合约 (只读)                                       │
│  • DeFi 协议直接读取价格，无需回调                                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 模式三：触发器 (NeoFlow)

用户注册触发器，TEE 监控条件并周期性调用回调：

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      触发器注册 (一次性)                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│  用户 → 用户合约.RegisterTrigger() → Gateway.RequestService("neoflow")   │
│                                    → NeoFlowService.OnRequest()           │
│                                                                              │
│  触发器类型:                                                                  │
│  • 时间触发: "每周五 00:00 UTC" (cron 表达式)                                 │
│  • 价格触发: "当 BTC > $100,000"                                             │
│  • 事件触发: "当合约 X 发出事件 Y"                                            │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│              服务层 (TEE) - 持续监控                                          │
├─────────────────────────────────────────────────────────────────────────────┤
│  循环检查所有已注册的触发器:                                                   │
│  • 时间触发: 比较当前时间                                                     │
│  • 价格触发: 检查 NeoFeeds 价格                                              │
│  • 事件触发: 监控区块链事件                                                   │
│  当条件满足 → 执行回调                                                        │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │ 条件满足
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         回调执行 (周期性)                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│  TEE → Gateway.FulfillRequest() → 用户合约.Callback()                        │
│  (如: 每周五自动分发代币, 价格达标自动卖出等)                                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

**NeoFlow 触发器示例:**

| 触发器类型 | 示例 | 用例 |
|------------|------|------|
| 时间触发 | `cron: "0 0 * * FRI"` | 每周代币分发 |
| 价格触发 | `price: BTC > 100000` | 价格达标自动卖出 |
| 阈值触发 | `balance < 10 GAS` | 自动充值 Gas 银行 |
| 事件触发 | `event: LiquidityAdded` | 响应链上事件 |

## 合约说明

### ServiceLayerGateway

主入口合约，负责：

- **费用管理**: 用户预存 GAS，按服务类型收费
- **TEE 账户管理**: 注册/移除 TEE 主账户及公钥
- **服务注册**: 注册服务类型与合约地址映射
- **请求路由**: 将用户请求路由到对应服务合约
- **回调执行**: TEE 完成后回调用户合约
- **重放保护**: Nonce 机制防止重放攻击

**主要方法**:

| 方法 | 说明 | 调用者 |
|------|------|--------|
| `RequestService(serviceType, payload, callback)` | 请求服务 | 用户合约 |
| `FulfillRequest(requestId, result, nonce, signature)` | 完成请求 | TEE |
| `FailRequest(requestId, reason, nonce, signature)` | 标记失败 | TEE |
| `RegisterTEEAccount(account, pubKey)` | 注册 TEE | Admin |
| `RegisterService(type, contract)` | 注册服务 | Admin |
| `BalanceOf(account)` | 查询余额 | 任何人 |

**费用标准** (默认):

| 服务 | 费用 (GAS) |
|------|-----------|
| Oracle | 0.1 |
| VRF | 0.1 |
| NeoVault | 0.5 |
| NeoFeeds | 0.05 |
| NeoFlow | 0.2 |
| NeoCompute | 1.0 |

### OracleService

外部数据预言机服务：

- 接收 HTTP 请求参数 (URL, Method, Headers, JSONPath)
- 发出 `OracleRequest` 事件
- TEE 执行 HTTP 请求并返回结果

**事件**:
```csharp
event OracleRequest(requestId, userContract, url, method, headers, jsonPath)
event OracleFulfilled(requestId, result)
```

### VRFService

可验证随机数服务：

- 接收种子和请求数量
- 增强种子 (添加区块哈希)
- 存储 VRF 证明供链上验证

**事件**:
```csharp
event VRFRequest(requestId, userContract, seed, numWords)
event VRFFulfilled(requestId, randomWords, proof)
```

### NeoVaultService

**确定性共享种子隐私混币服务 (v4.1)**

基于确定性共享种子派生的隐私交易服务，池账户使用**普通单签地址**，与普通用户完全相同。

**核心架构**:
```
┌─────────────────────────────────────────────────────────────────┐
│              确定性共享种子架构 (Deterministic Shared Seed)       │
├─────────────────────────────────────────────────────────────────┤
│  Shared_Seed = HKDF(Master_Secret, TEE_Attestation_Hash)        │
│       │                                                          │
│       ├── Derive("m/pool/0") → PrivKey_0 → 普通单签地址_0        │
│       ├── Derive("m/pool/1") → PrivKey_1 → 普通单签地址_1        │
│       └── Derive("m/pool/N") → PrivKey_N → 普通单签地址_N        │
│                                                                  │
│  Master 和 TEE 都能独立派生相同的私钥                             │
│  地址是普通单签地址 (无多签指纹，与普通用户完全相同)               │
└─────────────────────────────────────────────────────────────────┘
```

**关键设计原则**:
- **无链上池账户注册** - 池账户完全离线管理，链上无任何痕迹
- **普通单签地址** - 池账户与普通用户地址完全相同，无法区分
- **无脚本指纹** - 不使用多签，避免被链上分析识别
- **确定性共享** - Master 和 TEE 通过共享种子独立派生相同密钥
- **Master 可恢复** - TEE 故障时，Master 可重建种子恢复所有账户

**种子派生流程**:
```
1. Master 持有 Master_Secret (离线安全存储)
2. TEE 生成证明，Master 验证后计算:
   Shared_Seed = HKDF-SHA256(Master_Secret, TEE_Attestation_Hash)
3. Shared_Seed 通过安全通道注入 TEE
4. 双方可独立派生相同私钥:
   PrivKey_i = Derive(Shared_Seed, "m/pool/i")
5. Address_i = 标准 Neo 单签地址 (从 PrivKey_i 生成)
```

**账户生成代码** (链下):
```csharp
// 1. 计算共享种子 (Master 和 TEE 都执行)
byte[] sharedSeed = HKDF.DeriveKey(
    HashAlgorithmName.SHA256,
    masterSecret,
    32,
    teeAttestationHash,
    Encoding.UTF8.GetBytes("neo-vault-pool")
);

// 2. 派生私钥 (index = 账户序号)
byte[] privateKey = DeriveChildKey(sharedSeed, $"m/pool/{index}");

// 3. 生成标准单签地址 (与普通用户完全相同)
ECPoint publicKey = GetPublicKey(privateKey);
UInt160 poolAddress = Contract.CreateSignatureRedeemScript(publicKey).ToScriptHash();
```

**恢复机制**:
```
TEE 故障时:
1. Master 使用 Master_Secret + 存储的 TEE_Attestation_Hash
2. Master 重建 Shared_Seed
3. Master 派生所有私钥，转移资金到新池账户
```

**混币时长选项**: 30分钟 / 1小时 / 24小时 / 7天

**流程**:
```
1. Admin: RegisterService(teePubKey) + DepositBond()
2. 用户: CreateRequest(encryptedTargets, mixOption) + GAS 支付
3. TEE: ClaimRequest(requestId, recipients[], amounts[], signature)
   → 合约释放资金到 TEE 指定的 HD 池账户 (无链上验证)
4. TEE: (链下) 池内随机转账 + 噪声交易
5. TEE: SubmitCompletion(requestId, outputsHash, signature) → 标记完成
6. 用户 (超时): ClaimRefundByUser(requestId) → 从保证金退款
```

**主要方法**:

| 方法 | 说明 | 调用者 |
|------|------|--------|
| `RegisterService(serviceId, teePubKey)` | 注册混币服务 | Admin |
| `DepositBond()` | 存入保证金 | 服务 |
| `CreateRequest(...)` | 创建混币请求 | 用户 |
| `ClaimRequest(...)` | 认领请求并分发资金 | TEE |
| `SubmitCompletion(...)` | 提交完成证明 | TEE |
| `ClaimRefundByUser(requestId)` | 超时退款 | 用户 |

**事件**:
```csharp
event ServiceRegistered(serviceId, teePubKey)
event BondDeposited(serviceId, amount, totalBond)
event RequestCreated(requestId, depositor, amount, mixOption, deadline)
event RequestClaimed(requestId, serviceId, recipientCount, claimTime)
event RequestCompleted(requestId, serviceId, outputsHash)
event RefundClaimed(requestId, user, amount)
event BondSlashed(serviceId, slashedAmount, remainingBond)
```

**隐私保护机制**:
- **无链上注册** - 池账户不在合约中注册，链上分析无法识别
- **HD 派生** - 每个账户使用独立公钥，无法关联
- **随机拆分** - 金额随机拆分防止关联分析
- **噪声交易** - 持续随机交易混淆真实活动
- **时间窗口** - 混币时长增加时间不确定性

**安全机制**:
- TEE 签名验证所有服务操作
- 保证金覆盖 outstanding 金额
- Master 可通过 HD 派生恢复任意账户
- Nonce 防重放保护
- 截止时间 + 7天安全缓冲期

## 用户合约示例

参见 `examples/ExampleConsumer.cs`：

```csharp
// 请求价格数据
public static BigInteger RequestPrice(string pair, string url, string jsonPath)
{
    UInt160 gateway = GetGateway();

    OraclePayload payload = new OraclePayload
    {
        Url = url,
        Method = "GET",
        JsonPath = jsonPath
    };

    byte[] payloadBytes = StdLib.Serialize(payload);

    return (BigInteger)Contract.Call(gateway, "requestService", CallFlags.All,
        new object[] { "oracle", payloadBytes, "onServiceCallback" });
}

// 回调处理
public static void OnServiceCallback(BigInteger requestId, bool success, byte[] result, string error)
{
    // 验证调用者是 Gateway
    if (Runtime.CallingScriptHash != GetGateway())
        throw new Exception("Only gateway can callback");

    // 处理结果...
}
```

## 部署步骤

1. **部署 Gateway**
   ```bash
   neo-go contract deploy -i ServiceLayerGateway.nef -m ServiceLayerGateway.manifest.json
   ```

2. **部署服务合约**
   ```bash
   neo-go contract deploy -i OracleService.nef -m OracleService.manifest.json
   neo-go contract deploy -i VRFService.nef -m VRFService.manifest.json
   neo-go contract deploy -i NeoVaultService.nef -m NeoVaultService.manifest.json
   ```

3. **配置 Gateway**
   ```bash
   # 注册 TEE 账户
   neo-go contract invokefunction <gateway> registerTEEAccount <tee_address> <tee_pubkey>

   # 注册服务
   neo-go contract invokefunction <gateway> registerService oracle <oracle_hash>
   neo-go contract invokefunction <gateway> registerService neorand <neorand_hash>
   neo-go contract invokefunction <gateway> registerService neovault <neovault_hash>
   ```

4. **配置服务合约**
   ```bash
   # 设置 Gateway 地址
   neo-go contract invokefunction <oracle> setGateway <gateway_hash>
   neo-go contract invokefunction <neorand> setGateway <gateway_hash>
   neo-go contract invokefunction <neovault> setGateway <gateway_hash>
   ```

## Go 集成

### 事件监听

```go
import "github.com/R3E-Network/service_layer/internal/chain"

// 创建监听器
listener := chain.NewEventListener(chain.ListenerConfig{
    Client:    client,
    Contracts: contractAddresses,
    StartBlock: startBlock,
})

// 注册处理器
listener.On("OracleRequest", func(event *chain.ContractEvent) error {
    req, err := chain.ParseOracleRequestEvent(event)
    if err != nil {
        return err
    }
    // 处理 Oracle 请求...
    return nil
})

listener.On("VRFRequest", func(event *chain.ContractEvent) error {
    req, err := chain.ParseVRFRequestEvent(event)
    if err != nil {
        return err
    }
    // 处理 VRF 请求...
    return nil
})

// 启动监听
listener.Start(ctx)
```

### TEE 回调

```go
// 创建 TEE Fulfiller
fulfiller := chain.NewTEEFulfiller(client, gatewayHash, teeWallet)

// 完成请求
txHash, err := fulfiller.FulfillRequest(ctx, requestID, result)

// 标记失败
txHash, err := fulfiller.FailRequest(ctx, requestID, "error reason")
```

## 安全考虑

1. **TEE 签名验证**: 所有回调必须包含有效的 TEE 签名
2. **Nonce 重放保护**: 每个 nonce 只能使用一次
3. **Gateway 权限**: 服务合约只接受 Gateway 调用
4. **费用预付**: 用户必须预存足够的 GAS
5. **暂停机制**: Admin 可暂停合约应对紧急情况

### NeoComputeService

机密计算服务：

- 接收加密的输入数据
- TEE 解密、计算、加密输出
- 支持私密拍卖、投票、数据聚合等场景

**事件**:
```csharp
event NeoComputeRequest(requestId, userContract, computationType, encryptedInput, inputCommitment)
event NeoComputeFulfilled(requestId, encryptedOutput, outputCommitment)
```

**计算类型**:
| 类型 | 说明 |
|------|------|
| `aggregate` | 私密数据聚合 |
| `compare` | 私密比较 |
| `auction` | 密封拍卖 |
| `vote` | 私密投票 |

## 目录结构

```
contracts/
├── README.md                    # 本文档
├── build.sh                     # 编译脚本
├── common/
│   └── ServiceContractBase.cs   # 服务合约基类 (Gateway 模式)
├── gateway/
│   └── ServiceLayerGateway.cs   # 主入口合约
├── neooracle/
│   └── OracleService.cs         # Oracle 服务合约 (继承 ServiceContractBase)
├── neorand/
│   └── VRFService.cs            # VRF 服务合约 (继承 ServiceContractBase)
├── neocompute/
│   └── NeoComputeService.cs   # NeoCompute 服务合约 (继承 ServiceContractBase)
├── neovault/
│   └── NeoVaultService.cs          # NeoVault 服务合约 (独立架构)
├── neofeeds/
│   └── NeoFeedsService.cs      # NeoFeeds 服务合约 (独立架构)
├── neoflow/
│   └── NeoFlowService.cs     # NeoFlow 服务合约 (混合架构)
└── examples/
    ├── ExampleConsumer.cs       # 示例用户合约
    ├── VRFLottery.cs            # VRF 彩票示例
    ├── NeoVaultClient.cs           # NeoVault 客户端示例
    └── DeFiPriceConsumer.cs     # DeFi 价格消费者示例
```

### 合约架构分类

| 架构类型 | 合约 | 说明 |
|---------|------|------|
| **Gateway 模式** | OracleService, VRFService, NeoComputeService | 继承 ServiceContractBase，通过 Gateway 路由请求 |
| **独立架构** | NeoVaultService, NeoFeedsService | 有自己的 Admin/TEE 管理，不通过 Gateway |
| **混合架构** | NeoFlowService | 有 Gateway 但 SetGateway 需要 Admin 权限 |

## 编译要求

- .NET SDK 6.0+
- Neo.SmartContract.Framework 3.6+
- nccs (Neo Contract Compiler)

## 编译步骤

```bash
# 安装 Neo 合约编译器
dotnet tool install -g Neo.Compiler.CSharp

# 编译所有合约
cd contracts
./build.sh

# 或单独编译
cd gateway
nccs ServiceLayerGateway.cs
```

## 许可证

MIT License
