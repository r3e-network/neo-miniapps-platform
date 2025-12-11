# MegaLottery - Neo N3 去中心化彩票

基于 Neo N3 区块链和 Service Layer 基础设施的完全去中心化彩票 dApp。

## 特性

- **可证明公平**: 中奖号码使用 Service Layer VRF（可验证随机函数）生成，确保密码学安全的随机数
- **Quick Pick 随机选号**: 使用 Neo N3 内置 `Runtime.GetRandom()` syscall 生成随机号码
- **自动开奖**: 每日开奖由 Service Layer NeoFlow 服务自动触发
- **完全透明**: 所有操作记录在链上，完全可追溯
- **多级奖金**: MegaMillions 风格的奖金结构，头奖可累积
- **1分钟锁定期**: 开奖前1分钟停止售票，确保公平性

## 随机数生成机制

本彩票系统使用两种不同的随机数生成方式：

| 场景 | 方法 | 说明 |
|------|------|------|
| **Quick Pick 选号** | `Runtime.GetRandom()` | Neo N3 内置 syscall，基于区块数据生成伪随机数 |
| **中奖号码生成** | Service Layer VRF | 可验证随机函数，提供密码学证明的真随机数 |

这种设计确保：
- 用户选号时获得快速、便捷的随机体验
- 中奖号码具有可验证的公平性，任何人都可以验证

## 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        MegaLottery dApp                         │
├─────────────────────────────────────────────────────────────────┤
│  前端 (React + Vite + TypeScript + TailwindCSS)                 │
│  - 钱包连接 (NeoLine, OneGate, O3)                              │
│  - 号码选择器 UI                                                 │
│  - 彩票管理                                                      │
│  - 开奖结果展示                                                  │
├─────────────────────────────────────────────────────────────────┤
│  智能合约 (Neo N3 C#)                                           │
│  - 购票与存储                                                    │
│  - Quick Pick (Runtime.GetRandom)                               │
│  - 奖金分配                                                      │
│  - VRF 回调处理                                                  │
│  - NeoFlow 触发处理                                          │
├─────────────────────────────────────────────────────────────────┤
│  Service Layer 集成                                             │
│  ┌─────────────────┐  ┌─────────────────┐                      │
│  │   VRF Service   │  │ NeoFlow Svc  │                      │
│  │  (中奖号码生成)  │  │   (每日开奖)    │                      │
│  └─────────────────┘  └─────────────────┘                      │
└─────────────────────────────────────────────────────────────────┘
```

## 游戏规则

### 如何参与

1. 选择 5 个主号码（1-70）
2. 选择 1 个 Mega Ball（1-25）
3. 支付 2 GAS 购买彩票
4. 等待每日 UTC 00:00 开奖

### 选号方式

- **手动选号**: 在号码网格中点击选择您喜欢的号码
- **Quick Pick**: 点击"Quick Pick"按钮，系统使用 `Runtime.GetRandom()` 自动生成随机号码

### 奖金等级

| 匹配 | 奖金 | 概率 |
|------|------|------|
| 5 + Mega Ball | **头奖** (奖池 50%) | 1 / 302,575,350 |
| 5 个号码 | 奖池 20% | 1 / 12,607,306 |
| 4 + Mega Ball | 奖池 10% | 1 / 931,001 |
| 4 个或 3 + Mega | 奖池 10% | 1 / 38,792 |
| 3 个或 Mega Ball | 奖池 10% | 1 / 606 |

*10% 用于运营基金*

### 重要规则

- 每次开奖前 **1 分钟** 停止售票
- 奖金必须在 **30 天内** 领取
- 未领取的头奖将累积到下一期

## 项目结构

```
dapps/lottery/
├── contract/
│   ├── MegaLottery.cs        # Neo N3 智能合约
│   └── MegaLottery.csproj    # .NET 项目文件
├── frontend/
│   ├── src/
│   │   ├── components/       # React 组件
│   │   │   ├── Header.tsx        # 导航栏（含钱包连接）
│   │   │   ├── NumberPicker.tsx  # 号码选择器
│   │   │   ├── TicketCard.tsx    # 彩票卡片
│   │   │   └── Countdown.tsx     # 倒计时组件
│   │   ├── hooks/            # 自定义 Hooks
│   │   │   ├── useWallet.ts      # 钱包连接
│   │   │   └── useLottery.ts     # 彩票操作
│   │   ├── pages/            # 页面组件
│   │   │   ├── HomePage.tsx      # 首页
│   │   │   ├── BuyTicketPage.tsx # 购票页
│   │   │   ├── MyTicketsPage.tsx # 我的彩票
│   │   │   ├── ResultsPage.tsx   # 开奖结果
│   │   │   └── HowToPlayPage.tsx # 玩法说明
│   │   └── index.css         # 全局样式
│   ├── package.json          # 依赖配置
│   ├── vite.config.ts        # Vite 配置
│   └── tailwind.config.js    # TailwindCSS 配置
├── README.md                 # 英文文档
└── README_CN.md              # 中文文档（本文件）
```

## 安装与部署

### 环境要求

- Node.js 18+
- .NET SDK 7.0+（用于合约编译）
- Neo N3 钱包（含 GAS）

### 合约部署

1. **编译合约**:
```bash
cd contract
dotnet build
```

2. **部署到 Neo N3**:
```bash
# 使用 neo-express 或 neo-cli
neo-express contract deploy MegaLottery.nef
```

3. **初始化合约**:
```bash
# 设置 VRF 合约地址
neo-express contract invoke MegaLottery setVRFContract <VRF_CONTRACT_HASH>

# 设置 NeoFlow 合约地址
neo-express contract invoke MegaLottery setNeoFlowContract <AUTOMATION_CONTRACT_HASH>

# 注册每日触发器
neo-express contract invoke MegaLottery registerDailyTrigger
```

### 前端部署

1. **安装依赖**:
```bash
cd frontend
npm install
```

2. **配置环境变量**:
```bash
cp .env.example .env
# 编辑 .env 文件，填入合约哈希
```

环境变量说明：
```env
VITE_LOTTERY_CONTRACT=0x...  # MegaLottery 合约哈希
VITE_NETWORK=MainNet         # 网络：MainNet 或 TestNet
```

3. **启动开发服务器**:
```bash
npm run dev
```

4. **构建生产版本**:
```bash
npm run build
```

## Service Layer 集成

### VRF 集成（中奖号码）

合约向 Service Layer VRF 请求随机数用于生成中奖号码：

```csharp
// 请求 6 个随机数用于开奖
ByteString requestId = Contract.Call(vrfContract, "requestRandomness", CallFlags.All, new object[] {
    seed,           // 唯一种子
    6,              // 随机数数量
    contractHash,   // 回调合约
    200000          // 回调 Gas 限制
});
```

VRF 服务回调返回可验证的随机数：

```csharp
public static void FulfillRandomness(ByteString requestId, BigInteger[] randomWords)
{
    // 验证调用者是 VRF 合约
    // 将随机数转换为中奖号码
    // 完成开奖
}
```

### Quick Pick 实现（购票选号）

Quick Pick 使用 Neo N3 内置的 `Runtime.GetRandom()` syscall：

```csharp
public static BigInteger QuickPick()
{
    // 使用区块数据生成伪随机数
    ByteString entropy = Runtime.GetRandom().ToByteString();

    // 生成 5 个主号码 (1-70)
    byte[] mainNumbers = new byte[5];
    // ... 从 entropy 中提取不重复的号码

    // 生成 1 个 Mega Ball (1-25)
    byte megaNumber = (byte)((entropy[offset] % 25) + 1);

    // 购买彩票
    // ...
}
```

### NeoFlow 集成

每日开奖由 Service Layer NeoFlow 触发：

```csharp
// 注册基于时间的每日开奖触发器
Contract.Call(neoflowContract, "registerTrigger", CallFlags.All, new object[] {
    contractHash,       // 回调合约
    1,                  // 触发类型：时间
    "0 0 * * *",        // Cron 表达式：每日 UTC 00:00
    "unveilWinner",     // 回调方法
    0                   // 最大执行次数（0=无限）
});
```

## 前端技术栈

| 技术 | 用途 |
|------|------|
| React 18 | UI 框架 |
| TypeScript | 类型安全 |
| Vite | 构建工具 |
| TailwindCSS | 样式框架 |
| React Query | 数据获取与缓存 |
| React Router | 路由管理 |
| Lucide React | 图标库 |

### UI 设计特点

- **Glass Morphism**: 毛玻璃效果的现代设计
- **渐变色彩**: 金色主题的彩票球和按钮
- **响应式布局**: 完美支持移动端和桌面端
- **微动画**: 流畅的交互动画提升用户体验
- **骨架屏**: 优雅的加载状态展示

## 安全考虑

1. **VRF 验证**: 所有中奖号码都经过密码学验证
2. **锁定期**: 1分钟锁定期防止最后时刻操纵
3. **链上存储**: 所有彩票和结果存储在区块链上
4. **访问控制**: 管理员功能受 witness 检查保护
5. **重入保护**: 外部调用前完成状态变更

## 测试

### 合约测试
```bash
cd contract
dotnet test
```

### 前端测试
```bash
cd frontend
npm test
```

## 常见问题

### Q: Quick Pick 和 VRF 有什么区别？

**A**: Quick Pick 使用 `Runtime.GetRandom()` 为用户快速生成随机号码，这是一种基于区块数据的伪随机数，适合用户选号场景。而 VRF（可验证随机函数）用于生成中奖号码，提供密码学证明，确保开奖结果的公平性和不可预测性。

### Q: 为什么开奖前要锁定售票？

**A**: 1分钟的锁定期是为了防止在知道即将开奖的情况下进行最后时刻的操纵。这确保了所有参与者的公平性。

### Q: 奖金如何分配？

**A**: 奖池按以下比例分配：
- 50% - 头奖（5+Mega）
- 20% - 二等奖（5个号码）
- 10% - 三等奖（4+Mega）
- 10% - 四等奖（4个或3+Mega）
- 10% - 运营基金

### Q: 支持哪些钱包？

**A**: 目前支持以下 Neo N3 钱包：
- NeoLine（浏览器扩展）
- OneGate（多链钱包）
- O3 Wallet（移动端和桌面端）

## 许可证

MIT License - 详见 LICENSE 文件。

## 相关链接

- [Service Layer 文档](https://docs.servicelayer.io)
- [Neo N3 文档](https://docs.neo.org)
- [NeoLine 钱包](https://neoline.io)
- [OneGate 钱包](https://onegate.space)
- [O3 钱包](https://o3.network)
