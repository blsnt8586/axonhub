# 商业自助使用指南

AxonHub 商业账户采用“用户钱包”模型。Project 用于组织 API Key、请求、模型和价格规则，但 Project 不拥有独立钱包。

## 账户使用流程

1. 管理员开启公开注册后，从 `/sign-up` 注册。
2. 登录并选择工作空间。按照注册策略，新用户通常会获得默认 Project 和个人 API Key。
3. 打开“模型与价格”，确认当前 Project 存在可用模型和公开销售价格。
4. 在“API Keys”创建或限制个人 API Key。密钥明文只在创建或轮换时显示。
5. 使用 Playground，或者 OpenAI/Anthropic 兼容客户端发送请求。
6. 在“Usage”查看自己的请求和用量汇总。消费者页面不会暴露 Channel、上游账户、路由 Trace、上游成本或平台利润。
7. 在“钱包与账单”查看余额、充值、订单、订阅、兑换码、邀请返佣、通知、账本流水和用量扣费。

## 计费规则

- 充值、兑换入账、订阅购买、退款和用量扣费都写入同一个用户钱包账本。
- Project 或全局价格规则只决定用户销售价，不改变钱包归属。
- 当订阅覆盖当前 Project 和模型时，优先消耗订阅额度，再按配置回退用户钱包。
- 聚合图表可能稍后刷新，但请求详情、UsageBillingRecord 和 LedgerTransaction 使用同一个最终结算金额。

## 权限与停用

所有处于 Active Project 的成员都自动获得 AI 消费、管理本人个人 Key、查看本人用量的能力，不需要写入任何系统管理 scope。

只有 Project owner 或拥有对应 Project scope 的成员才会看到 Project administration。Project owner 身份不会授予系统管理权限。

管理员停用用户后，该用户的密码登录、已有 JWT 会话，以及其 `personal` 和历史 `user` 类型 API Key 都会被拒绝。Project 的 `service_account` 属于项目运行凭据，如需停用必须单独处理。

## 无渠道安装

如果系统没有启用任何可提供模型的 Channel，Home 和 Playground 会显示明确的“无可用模型”状态。管理员需要先启用 Channel 并配置公开价格，用户才能调用 AI。
