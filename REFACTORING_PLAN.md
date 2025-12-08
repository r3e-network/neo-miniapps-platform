# Service Layer 项目重构计划

## 目标

将每个服务重构为自包含的模块，使服务的所有相关代码（实现、合约、数据库、链交互）都在同一个文件夹下，便于维护和理解。

## 当前结构

```
service_layer/
├── services/
│   ├── oracle/          # 服务实现
│   ├── vrf/
│   ├── mixer/
│   ├── datafeeds/
│   ├── automation/
│   ├── confidential/
│   ├── accountpool/
│   └── secrets/
├── contracts/
│   ├── oracle/          # 智能合约 (分散)
│   ├── vrf/
│   ├── mixer/
│   ├── datafeeds/
│   ├── automation/
│   ├── confidential/
│   ├── gateway/         # 共享
│   ├── common/          # 共享
│   └── examples/        # 共享
├── internal/
│   ├── chain/           # 链交互 (分散)
│   │   ├── contracts_oracle.go (不存在)
│   │   ├── contracts_vrf.go
│   │   ├── contracts_mixer.go
│   │   ├── contracts_datafeeds.go
│   │   ├── contracts_automation.go
│   │   ├── contracts_gateway.go    # 共享
│   │   ├── contracts_common.go     # 共享
│   │   ├── contracts_fulfiller.go  # 共享
│   │   └── contracts_parsers.go    # 共享
│   └── database/        # 数据库交互 (分散)
│       ├── supabase_vrf.go
│       ├── supabase_mixer.go
│       ├── supabase_automation.go
│       ├── supabase_accountpool.go
│       ├── supabase_secrets.go
│       ├── supabase_client.go      # 共享
│       ├── supabase_repository.go  # 共享
│       └── supabase_models.go      # 共享
```

## 目标结构

```
service_layer/
├── services/
│   ├── oracle/
│   │   ├── marble/              # 服务实现 (TEE/Marble 运行时)
│   │   │   ├── service.go
│   │   │   ├── config.go
│   │   │   └── types.go
│   │   ├── contract/            # 智能合约
│   │   │   └── OracleService.cs
│   │   ├── chain/               # 链交互
│   │   │   └── contract.go      # (新建，目前不存在)
│   │   └── supabase/            # 数据库交互
│   │       └── repository.go    # (新建，目前不存在)
│   │
│   ├── vrf/
│   │   ├── marble/
│   │   │   ├── vrf.go
│   │   │   └── vrf_test.go
│   │   ├── contract/
│   │   │   └── VRFService.cs
│   │   ├── chain/
│   │   │   └── contract.go      # 从 contracts_vrf.go 移动
│   │   └── supabase/
│   │       └── repository.go    # 从 supabase_vrf.go 移动
│   │
│   ├── mixer/
│   │   ├── marble/
│   │   │   ├── service.go
│   │   │   ├── mixing.go
│   │   │   ├── pool.go
│   │   │   ├── handlers.go
│   │   │   ├── types.go
│   │   │   └── mixer_test.go
│   │   ├── contract/
│   │   │   └── MixerService.cs
│   │   ├── chain/
│   │   │   └── contract.go      # 从 contracts_mixer.go 移动
│   │   └── supabase/
│   │       └── repository.go    # 从 supabase_mixer.go 移动
│   │
│   ├── datafeeds/
│   │   ├── marble/
│   │   │   ├── datafeeds.go
│   │   │   ├── config.go
│   │   │   ├── chainlink.go
│   │   │   └── datafeeds_test.go
│   │   ├── contract/
│   │   │   └── DataFeedsService.cs
│   │   ├── chain/
│   │   │   └── contract.go      # 从 contracts_datafeeds.go 移动
│   │   └── supabase/
│   │       └── repository.go    # (新建，目前不存在)
│   │
│   ├── automation/
│   │   ├── marble/
│   │   │   ├── automation_service.go
│   │   │   ├── automation_handlers.go
│   │   │   ├── automation_triggers.go
│   │   │   ├── automation_types.go
│   │   │   └── automation_test.go
│   │   ├── contract/
│   │   │   └── AutomationService.cs
│   │   ├── chain/
│   │   │   └── contract.go      # 从 contracts_automation.go 移动
│   │   └── supabase/
│   │       └── repository.go    # 从 supabase_automation.go 移动
│   │
│   ├── confidential/
│   │   ├── marble/
│   │   │   ├── confidential.go
│   │   │   └── confidential_test.go
│   │   ├── contract/
│   │   │   └── ConfidentialService.cs
│   │   ├── chain/
│   │   │   └── contract.go      # (新建，目前不存在)
│   │   └── supabase/
│   │       └── repository.go    # (新建，目前不存在)
│   │
│   ├── accountpool/
│   │   ├── marble/
│   │   │   ├── service.go
│   │   │   ├── pool.go
│   │   │   ├── signing.go
│   │   │   ├── handlers.go
│   │   │   ├── types.go
│   │   │   └── accountpool_test.go
│   │   ├── contract/            # (无合约)
│   │   ├── chain/
│   │   │   └── contract.go      # (新建，目前不存在)
│   │   └── supabase/
│   │       └── repository.go    # 从 supabase_accountpool.go 移动
│   │
│   └── secrets/
│       ├── marble/
│       │   ├── service.go
│       │   ├── handlers.go
│       │   ├── types.go
│       │   └── service_test.go
│       ├── contract/            # (无合约)
│       ├── chain/
│       │   └── contract.go      # (新建，目前不存在)
│       └── supabase/
│           └── repository.go    # 从 supabase_secrets.go 移动
│
├── contracts/                   # 保留共享合约
│   ├── gateway/
│   │   └── ServiceLayerGateway.cs
│   ├── common/
│   │   └── ServiceContractBase.cs
│   ├── examples/
│   │   ├── ExampleConsumer.cs
│   │   ├── VRFLottery.cs
│   │   ├── MixerClient.cs
│   │   └── DeFiPriceConsumer.cs
│   └── build/                   # 编译输出
│
├── internal/
│   ├── chain/                   # 保留共享链交互
│   │   ├── client.go
│   │   ├── wallet.go
│   │   ├── params.go
│   │   ├── listener.go
│   │   ├── contracts_gateway.go
│   │   ├── contracts_common.go
│   │   ├── contracts_fulfiller.go
│   │   └── contracts_parsers.go
│   └── database/                # 保留共享数据库
│       ├── supabase_client.go
│       ├── supabase_repository.go
│       ├── supabase_models.go
│       ├── repository_interface.go
│       ├── mock_repository.go
│       └── (其他共享文件)
```

## 重构步骤

### 阶段 1: 创建目录结构 (低风险)

为每个服务创建新的子目录结构：

```bash
# 为每个服务创建 marble/, contract/, chain/, supabase/ 目录
for service in oracle vrf mixer datafeeds automation confidential accountpool secrets; do
    mkdir -p services/$service/{marble,contract,chain,supabase}
done
```

### 阶段 2: 移动服务实现到 marble/ (中风险)

| 服务 | 源文件 | 目标 |
|------|--------|------|
| oracle | services/oracle/*.go | services/oracle/marble/ |
| vrf | services/vrf/*.go | services/vrf/marble/ |
| mixer | services/mixer/*.go | services/mixer/marble/ |
| datafeeds | services/datafeeds/*.go | services/datafeeds/marble/ |
| automation | services/automation/*.go | services/automation/marble/ |
| confidential | services/confidential/*.go | services/confidential/marble/ |
| accountpool | services/accountpool/*.go | services/accountpool/marble/ |
| secrets | services/secrets/*.go | services/secrets/marble/ |

**注意**: 需要更新 package 声明和 import 路径

### 阶段 3: 移动智能合约到 contract/ (低风险)

| 服务 | 源文件 | 目标 |
|------|--------|------|
| oracle | contracts/oracle/OracleService.cs | services/oracle/contract/ |
| vrf | contracts/vrf/VRFService.cs | services/vrf/contract/ |
| mixer | contracts/mixer/MixerService.cs | services/mixer/contract/ |
| datafeeds | contracts/datafeeds/DataFeedsService.cs | services/datafeeds/contract/ |
| automation | contracts/automation/AutomationService.cs | services/automation/contract/ |
| confidential | contracts/confidential/ConfidentialService.cs | services/confidential/contract/ |

**保留在 contracts/**: gateway/, common/, examples/, build/

### 阶段 4: 移动链交互代码到 chain/ (高风险)

| 服务 | 源文件 | 目标 |
|------|--------|------|
| vrf | internal/chain/contracts_vrf.go | services/vrf/chain/contract.go |
| mixer | internal/chain/contracts_mixer.go | services/mixer/chain/contract.go |
| datafeeds | internal/chain/contracts_datafeeds.go | services/datafeeds/chain/contract.go |
| automation | internal/chain/contracts_automation.go | services/automation/chain/contract.go |

**保留在 internal/chain/**: client.go, wallet.go, params.go, listener.go, contracts_gateway.go, contracts_common.go, contracts_fulfiller.go, contracts_parsers.go

**注意**:
- 需要更新 package 声明 (从 `package chain` 到 `package chain` 或服务特定包名)
- 需要更新所有 import 路径
- 共享类型需要从 internal/chain 导入

### 阶段 5: 移动数据库代码到 supabase/ (高风险)

| 服务 | 源文件 | 目标 |
|------|--------|------|
| vrf | internal/database/supabase_vrf.go | services/vrf/supabase/repository.go |
| mixer | internal/database/supabase_mixer.go | services/mixer/supabase/repository.go |
| automation | internal/database/supabase_automation.go | services/automation/supabase/repository.go |
| accountpool | internal/database/supabase_accountpool.go | services/accountpool/supabase/repository.go |
| secrets | internal/database/supabase_secrets.go, supabase_secret_permissions.go | services/secrets/supabase/repository.go |

**保留在 internal/database/**: supabase_client.go, supabase_repository.go, supabase_models.go, repository_interface.go, mock_repository.go, supabase_apikeys.go, supabase_gasbank.go, supabase_oauth.go, supabase_sessions.go, supabase_wallets.go

### 阶段 6: 更新 Import 路径 (高风险)

需要更新所有引用移动文件的 import 路径：

```go
// 旧路径
import "github.com/R3E-Network/service_layer/internal/chain"
import "github.com/R3E-Network/service_layer/internal/database"

// 新路径 (服务特定)
import "github.com/R3E-Network/service_layer/services/vrf/chain"
import "github.com/R3E-Network/service_layer/services/vrf/supabase"
```

### 阶段 7: 更新编译脚本 (低风险)

更新 contracts/build.sh 以反映新的合约位置：

```bash
# 更新合约路径
contracts=(
    "../services/oracle/contract/OracleService"
    "../services/vrf/contract/VRFService"
    "../services/mixer/contract/MixerService"
    # ...
)
```

### 阶段 8: 验证和测试 (必需)

1. 运行 `go build ./...` 确保编译通过
2. 运行 `go test ./...` 确保所有测试通过
3. 验证合约编译 `cd contracts && ./build.sh`

## 文件移动清单

### Oracle 服务
```
services/oracle/service.go      → services/oracle/marble/service.go
services/oracle/config.go       → services/oracle/marble/config.go
services/oracle/types.go        → services/oracle/marble/types.go
services/oracle/service_test.go → services/oracle/marble/service_test.go
contracts/oracle/OracleService.cs → services/oracle/contract/OracleService.cs
(无 chain 文件)
(无 supabase 文件)
```

### VRF 服务
```
services/vrf/vrf.go             → services/vrf/marble/vrf.go
services/vrf/vrf_test.go        → services/vrf/marble/vrf_test.go
contracts/vrf/VRFService.cs     → services/vrf/contract/VRFService.cs
internal/chain/contracts_vrf.go → services/vrf/chain/contract.go
internal/database/supabase_vrf.go → services/vrf/supabase/repository.go
```

### Mixer 服务
```
services/mixer/service.go       → services/mixer/marble/service.go
services/mixer/mixing.go        → services/mixer/marble/mixing.go
services/mixer/pool.go          → services/mixer/marble/pool.go
services/mixer/handlers.go      → services/mixer/marble/handlers.go
services/mixer/types.go         → services/mixer/marble/types.go
services/mixer/mixer_test.go    → services/mixer/marble/mixer_test.go
contracts/mixer/MixerService.cs → services/mixer/contract/MixerService.cs
internal/chain/contracts_mixer.go → services/mixer/chain/contract.go
internal/database/supabase_mixer.go → services/mixer/supabase/repository.go
```

### DataFeeds 服务
```
services/datafeeds/datafeeds.go      → services/datafeeds/marble/datafeeds.go
services/datafeeds/config.go         → services/datafeeds/marble/config.go
services/datafeeds/chainlink.go      → services/datafeeds/marble/chainlink.go
services/datafeeds/config_test.go    → services/datafeeds/marble/config_test.go
services/datafeeds/datafeeds_test.go → services/datafeeds/marble/datafeeds_test.go
contracts/datafeeds/DataFeedsService.cs → services/datafeeds/contract/DataFeedsService.cs
internal/chain/contracts_datafeeds.go → services/datafeeds/chain/contract.go
(无 supabase 文件)
```

### Automation 服务
```
services/automation/automation_service.go  → services/automation/marble/automation_service.go
services/automation/automation_handlers.go → services/automation/marble/automation_handlers.go
services/automation/automation_triggers.go → services/automation/marble/automation_triggers.go
services/automation/automation_types.go    → services/automation/marble/automation_types.go
services/automation/automation_test.go     → services/automation/marble/automation_test.go
contracts/automation/AutomationService.cs  → services/automation/contract/AutomationService.cs
internal/chain/contracts_automation.go     → services/automation/chain/contract.go
internal/database/supabase_automation.go   → services/automation/supabase/repository.go
```

### Confidential 服务
```
services/confidential/confidential.go      → services/confidential/marble/confidential.go
services/confidential/confidential_test.go → services/confidential/marble/confidential_test.go
contracts/confidential/ConfidentialService.cs → services/confidential/contract/ConfidentialService.cs
(无 chain 文件)
(无 supabase 文件)
```

### AccountPool 服务
```
services/accountpool/service.go         → services/accountpool/marble/service.go
services/accountpool/pool.go            → services/accountpool/marble/pool.go
services/accountpool/signing.go         → services/accountpool/marble/signing.go
services/accountpool/handlers.go        → services/accountpool/marble/handlers.go
services/accountpool/types.go           → services/accountpool/marble/types.go
services/accountpool/accountpool_test.go → services/accountpool/marble/accountpool_test.go
(无合约)
(无 chain 文件)
internal/database/supabase_accountpool.go → services/accountpool/supabase/repository.go
```

### Secrets 服务
```
services/secrets/service.go      → services/secrets/marble/service.go
services/secrets/handlers.go     → services/secrets/marble/handlers.go
services/secrets/types.go        → services/secrets/marble/types.go
services/secrets/service_test.go → services/secrets/marble/service_test.go
(无合约)
(无 chain 文件)
internal/database/supabase_secrets.go → services/secrets/supabase/repository.go
internal/database/supabase_secret_permissions.go → services/secrets/supabase/permissions.go
```

## 风险评估

| 阶段 | 风险等级 | 原因 |
|------|----------|------|
| 1. 创建目录 | 低 | 仅创建空目录 |
| 2. 移动服务实现 | 中 | 需要更新 package 和 import |
| 3. 移动合约 | 低 | C# 文件，无 Go 依赖 |
| 4. 移动链交互 | 高 | 多处引用，需要仔细更新 import |
| 5. 移动数据库代码 | 高 | 多处引用，需要仔细更新 import |
| 6. 更新 Import | 高 | 全局搜索替换，容易遗漏 |
| 7. 更新编译脚本 | 低 | 简单路径更新 |
| 8. 验证测试 | 必需 | 确保重构正确 |

## 建议执行顺序

1. **先做低风险的**: 阶段 1, 3, 7
2. **再做中风险的**: 阶段 2
3. **最后做高风险的**: 阶段 4, 5, 6
4. **每个阶段后验证**: 阶段 8

## 回滚策略

在开始重构前：
```bash
git checkout -b refactor/service-structure
```

每个阶段完成后提交：
```bash
git add -A && git commit -m "refactor: [阶段描述]"
```

如果出现问题：
```bash
git checkout master
git branch -D refactor/service-structure
```

## 预计工作量

| 阶段 | 预计时间 |
|------|----------|
| 阶段 1-3 | 30 分钟 |
| 阶段 4-5 | 2-3 小时 |
| 阶段 6 | 1-2 小时 |
| 阶段 7-8 | 30 分钟 |
| **总计** | **4-6 小时** |

## 待确认问题

1. **Package 命名**: 移动后的文件应该使用什么 package 名？
   - 选项 A: 保持原名 (如 `package chain`)
   - 选项 B: 使用服务名 (如 `package vrfchain`)
   - 选项 C: 使用通用名 (如 `package contract`)

2. **共享代码处理**: 某些服务没有对应的 chain 或 supabase 文件，是否需要创建空文件或占位符？

3. **测试文件**: 测试文件是否也移动到 marble/ 目录？

4. **合约编译输出**: 编译后的 .nef 和 .manifest.json 文件放在哪里？
   - 选项 A: 保持在 contracts/build/
   - 选项 B: 移动到各服务的 contract/build/
