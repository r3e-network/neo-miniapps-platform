# Neo N3 小应用平台（更新：支付仅 GAS，治理仅 NEO）

> 约束：支付/结算 **仅 GAS**；治理 **仅 NEO**（不再支持 bNEO）；服务层基于 **MarbleRun + EGo (SGX TEE)**；网关/数据用 **Supabase**；宿主前端 **Vercel + Next.js + 微前端**；包含高频 **Datafeed（≥0.1% 变动推送）**、VRF、Oracle、机密计算、自动化。
>
> 实现备注（本仓库）：为减少重复代码，`VRF` 功能可通过在 `compute-service`（`neocompute`）内执行受限脚本来提供；如需独立 `vrf-service`，可在此规范基础上再拆分为单独服务。

---

## 0. 技术栈
- 链上：Neo N3；合约 C#（neo-devpack-dotnet）；本地链 neo-express；测试框架 Neo.TestingFramework。
- 前端链交互：neon-js + NeoLine/O3/OneGate。
- 服务层 (TEE)：MarbleRun + EGo（attested TLS）。
- 网关/数据：Supabase（Auth/PG/Storage/Edge Functions，RLS 严隔离）；Edge 仅鉴权/限流/路由。
- 宿主前端：Next.js（Vercel）+ Module Federation/iframe + 严格 CSP；postMessage 白名单。
- 自动化：Keeper/Automation 在 TEE 内轮询或事件触发。
- CI/CD：GitHub Actions（合约/TEE/前端/Edge 构建与安全扫描）。

---

## 1. 目录结构（Mono-repo）
```
neo-miniapp-platform/
├─ contracts/
│  ├─ PaymentHub/       # 仅 GAS 收付分账
│  ├─ Governance/       # 仅 NEO 治理/质押/投票
│  ├─ PriceFeed/        # Datafeed 上链存证（0.1% 触发）
│  ├─ RandomnessLog/    # 随机数+TEE 报告存证
│  ├─ AppRegistry/      # 应用上架/状态/allowlist(MRSIGNER/合约)
│  └─ AutomationAnchor/ # 自动化任务登记/防重放
│
├─ services/            # TEE 服务层（MarbleRun + EGo）
│  ├─ oracle-gateway/   # 隐私预言机/Datafeed 拉取+聚合
│  ├─ datafeed-service/ # 高频价格推送（≥0.1% 变动），含阈值/去抖
│  ├─ compute-service/  # 机密计算（可选 wasm/脚本；可提供 RNG/VRF 脚本）
│  ├─ automation-service/# Keeper/定时/事件触发
│  ├─ tx-proxy/         # 交易签名/广播，资产&方法白名单
│  └─ marblerun/        # policy.json / manifest.json / CA
│
├─ platform/
│  ├─ host-app/         # 前端宿主（Next.js/Vercel）
│  ├─ sdk/              # JS SDK（payGAS/vote/rng/datafeed）
│  ├─ edge/             # Supabase Edge（鉴权/限流/路由）
│  ├─ rls/              # Supabase RLS 策略 SQL
│  └─ admin-console/    # 审核/运维后台（可选）
│
├─ miniapps/
│  ├─ builtin/          # 官方内置：game-lite, raffle, automation-demo, price-ticker
│  └─ community/        # 模板/示例
│      └─ template/     # Manifest + 前端脚手架
│
├─ infra/
│  ├─ neo-express/      # 本地链配置
│  ├─ docker/           # dev/test 容器编排（Supabase 等）
│  └─ ci/               # GitHub Actions 工作流
│
└─ docs/
   ├─ manifest-spec.md
   ├─ sdk-guide.md
   ├─ service-api.md
   └─ security-checklist.md
```

---

## 2. 合约要点（资产白名单）
- 常量：`GAS`、`NEO`。
- PaymentHub：仅 GAS；pay/refund；分账配置；限额/频率。
- Governance：仅 NEO；stake/unstake/vote；治理平台参数（费率、白名单）。
- PriceFeed：`symbol -> { round_id, price, ts, attestation_hash, sourceset_id }`；TEE 签名/测量校验；round_id 单调。
- RandomnessLog：`requestId -> randomness + attestationHash + timestamp`。
- AppRegistry：`app_id -> manifest_hash/status/allowlist`（合约/MRSIGNER）。
- AutomationAnchor：登记自动化任务（target/method/trigger/gasLimit），记录 nonce/txHash 防重放。

---

## 3. 服务层（MarbleRun + EGo）
- datafeed-service：多源聚合，触发阈值 0.1%，hysteresis 0.08%，最小发布间隔 2~5s，最大发布频率/符号（如每分钟 ≤30）；异常偏差需多源一致/二次确认；写 PriceFeed。
- oracle-gateway：外部数据抓取/聚合/校验 → 回调链上或存证。
- compute-service：受限脚本/wasm 计算 → 结果+报告 → 可回调链上。
- randomness（RNG/VRF）：通过 compute-service 执行受限脚本提供 → (randomness, attestation) → RandomnessLog 或回调。
- automation-service：事件/时间触发 → 调用目标合约（allowlist）。
- tx-proxy：签名/广播；资产仅 GAS/NEO；方法白名单；mTLS；防重放/额度检查。
- marblerun：policy/manifest 管理 MRSIGNER/MRENCLAVE、证书与密钥注入、轮换。

---

## 4. 平台宿主 & SDK
- 宿主：Next.js + Module Federation/iframe；严格 CSP（default-src 'self'; 禁 eval/外域脚本）；postMessage 白名单。
- SDK（`window.MiniAppSDK`）示例：
```ts
await window.MiniAppSDK.wallet.getAddress();
await window.MiniAppSDK.payments.payGAS(appId, "1.5", "entry fee");
const { randomness, reportHash } = await window.MiniAppSDK.rng.requestRandom(appId);
await window.MiniAppSDK.governance.vote(appId, proposalId, "10", "NEO");
const price = await window.MiniAppSDK.datafeed.getPrice("BTC-USD");
```
- 小程序禁止自构交易；敏感操作经 SDK → Edge → TEE → 链上。

---

## 5. Supabase（Auth/PG/RLS/Edge）
- Auth：登录；地址绑定（一次签名）。
- RLS：按 `user_id + app_id` 隔离，默认拒绝；写入仅服务角色（密钥在 TEE）。
- Edge：鉴权、nonce、防重放、限流、资产预检（支付仅 GAS；治理仅 NEO）；mTLS 转发到 TEE。
- Storage：按 `app_id` 路径隔离对象。

---

## 6. Manifest 关键字段示例
```json
{
  "app_id": "your-app-id",
  "entry_url": "https://cdn.example.com/apps/demo/index.html",
  "name": "Demo Miniapp",
  "version": "1.0.0",
  "developer_pubkey": "0x...",
  "permissions": { "wallet": ["read-address"], "payments": true, "governance": false, "randomness": true, "datafeed": true, "storage": ["kv"] },
  "assets_allowed": ["GAS"],
  "governance_assets_allowed": ["NEO"],
  "limits": { "max_gas_per_tx": "5", "daily_gas_cap_per_user": "20", "governance_cap": "100" },
  "contracts_needed": ["PaymentHub","RandomnessLog","PriceFeed"],
  "sandbox_flags": ["no-eval","strict-csp"],
  "attestation_required": true
}
```

---

## 7. Datafeed 设计（0.1% 触发）
- 阈值：发布 0.1%；回撤 0.08%（hysteresis）。
- 频控：最小发布间隔 2~5s；最大发布频率/符号（如每分钟 ≤30）。
- 防抖/去噪：中位数/加权 + EWMA；偏离上次链上值过大需多源一致。
- 上链：写 PriceFeed；round_id 单调；签名+attestation 校验。
- 计费：按订阅/调用量在 PaymentHub 扣 GAS；Edge/TEE 做额度预检。

---

## 8. 安全与合规
- 四层校验：SDK → Edge → TEE → 合约（资产/方法白名单一致）。
- TEE 证明：MarbleRun 管理测量；验证端点；合约可存 attestation 哈希。
- 限额/频率：per app_id / user_id；治理单独额度；大额可二次确认/多签。
- 防重放/审计：request_id 去重；链上事件 + PG 审计；敏感日志不出 enclave。
- 合规：对彩票/预测市场类做地理/年龄/KYC（前端拦截 + 后端校验）。

---

## 9. CI/CD（GitHub Actions 建议）
- 合约：C# 构建+单测；测试网部署脚本。
- EGo：构建可执行文件；产出测量值；生成 MarbleRun policy/manifest；安全扫描。
- 前端/SDK/Edge：构建与 lint；CSP/依赖漏洞扫描。
- 集成测：neo-express + EGo 仿真 + Supabase 本地，跑 Oracle/VRF/Datafeed/支付/治理/自动化全链路。

---

## 10. MVP 里程碑
1) 测试网部署：PaymentHub(GAS)、Governance(NEO)、PriceFeed、RandomnessLog、AppRegistry、AutomationAnchor。
2) 服务：compute-service（含 RNG/VRF 脚本） + datafeed-service(0.1% 阈值) + tx-proxy（EGo 仿真），MarbleRun dev policy。
3) 平台：Next.js 宿主 + SDK + iframe；Edge 鉴权/限流；Supabase 本地/云。
4) 内置小程序：`game-lite`、`raffle`、`automation-demo`、`price-ticker`。
5) CI 打通：合约单测、EGo 构建、前端/Edge 构建与安全检查。

---

## 11. 开发者（小程序）流程
1) 用模板创建前端；填 manifest（assets_allowed 仅 GAS，governance_assets_allowed 仅 NEO）。
2) 接入 `window.MiniAppSDK`（payGAS / rng / datafeed / vote）。
3) 本地：neo-express + Supabase 本地 + SDK Mock/TEE 仿真，自测支付/随机数/价格订阅。
4) 打包前端，生成 manifest_hash，提交审核；（若有）合约部署测试网。
5) 审核通过，上架目录；平台签名 manifest。

---

## 12. 可立即提供的文件（按需索取）
- 合约骨架（C#）：PaymentHub / Governance(NEO-only) / PriceFeed / RandomnessLog / AppRegistry / AutomationAnchor
- EGo/MarbleRun：policy.json / manifest.json 示例；compute-service（含 RNG/VRF 脚本） & datafeed-service & tx-proxy 壳
- Supabase：RLS SQL + Edge 路由模板（鉴权/限流/资产预检/mTLS 转发）
- 前端：Next.js 宿主 + Module Federation 脚手架 + JS SDK（payGAS/vote/rng/datafeed）
- Dev：neo-express 配置、一键本地脚本、Supabase docker-compose

请告知优先要的模块，我将直接给出对应文件内容。
