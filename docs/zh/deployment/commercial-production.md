# 商业版生产部署

本文适用于当前商业化 fork。核心业务约束是：用户钱包是付款主体，项目级价格规则只决定计价方式；AxonHub 原有 Channel 调度、账号池、熔断和跨渠道切换继续负责上游执行。

## 发布门禁

提交发布版本前运行：

```bash
./scripts/e2e/commercial-release-gate.sh --quick
./scripts/e2e/commercial-release-gate.sh --full
```

`--quick` 覆盖核心 Go 回归、GraphQL 租户隔离、真实网关计费以及优惠、订阅、模拟支付和返佣浏览器闭环。`--full` 额外覆盖桌面/移动端、日志脱敏、全新 PostgreSQL、Docker 镜像、备份恢复和历史商业数据库升级矩阵。

查看清单但不执行：

```bash
./scripts/e2e/commercial-release-gate.sh --full --plan
```

## 配置准备

从模板创建部署环境文件，并将实际文件放在 Git 仓库之外或加入密钥管理系统：

```bash
cp deploy/commercial.env.example /secure/path/axonhub-commercial.env
chmod 600 /secure/path/axonhub-commercial.env
```

至少替换：

- `DB_PASSWORD`：PostgreSQL 独立随机密码。
- `AXONHUB_PAYMENT_SECRET_KEY`：支付渠道密钥的静态加密根密钥。首次保存真实支付渠道前必须设置，此后不得随意更换，否则已有密文无法解密。
- `AXONHUB_BIND_ADDRESS`：有反向代理时保持 `127.0.0.1`；需要直接对外暴露时再改为受防火墙保护的地址。
- `POSTGRES_BIND_ADDRESS`：默认保持 `127.0.0.1`，生产数据库不应直接暴露公网。

验证 Compose 最终配置：

```bash
./scripts/e2e/commercial-deployment-config-test.sh
docker compose --env-file /secure/path/axonhub-commercial.env config --quiet
```

## 数据库迁移

当前应用启动时执行 Ent 自动迁移。生产首次上线或版本升级采用以下顺序：

1. 停止写流量并将应用缩容到零，记录当前镜像摘要和 Git tag。
2. 使用 `pg_dump --format=custom` 创建包含商业表的备份，并在隔离数据库验证可恢复。
3. 保持 `AXONHUB_DB_DISABLE_AUTO_MIGRATION=false`，只启动一个新版本应用实例。
4. 等待 `/health` 成功，检查迁移日志、商业表、Owner 登录和钱包查询。
5. 再扩容应用实例并恢复业务流量。
6. 只有在外部迁移流程完全接管 schema 后，才设置 `AXONHUB_DB_DISABLE_AUTO_MIGRATION=true`。

不要让多个新版本实例并发执行首次迁移。真实生产数据量的迁移耗时和锁等待必须在发布窗口前用生产数据副本测量。

## 计费启用顺序

首次商业上线不要从 `disabled` 直接切到 `enforce`：

1. `disabled`：验证注册、登录、钱包、价格规则、支付渠道、订阅和管理员页面。
2. `warn`：发送真实 API 流量，观察缺失价格规则、余额不足、计费 Outbox、钱包 Hold 和 UsageBillingRecord；请求仍可继续路由。
3. 修复价格、余额、订阅和失败 Outbox，确认同一请求只有一条最终用量扣费。
4. 创建数据库备份、Git tag 和可回退镜像。
5. `enforce`：验证余额不足在上游调用前返回 HTTP 402，同时验证正常余额、订阅优先和跨渠道重试。

推荐初始值已经写入 `deploy/commercial.env.example`：`subject=user`、`currency=CNY`、`allow_negative=false`、`block_when_no_price_rule=true`。

## 启动与检查

```bash
docker compose --env-file /secure/path/axonhub-commercial.env up -d --build
docker compose --env-file /secure/path/axonhub-commercial.env ps
curl --fail http://127.0.0.1:8090/health
```

上线后检查：

- Owner 与普通用户登录、注册策略和角色隔离。
- 用户钱包、账本、充值订单、支付事件、订阅、优惠码和返佣。
- 项目价格规则影响金额，但最终扣用户钱包。
- 账号池可调度状态、健康度、冷却时间、配额、切换历史和渠道熔断。
- 应用日志、反向代理日志、Docker 日志驱动和外部采集器中没有完整 API Key、Cookie、支付密钥或数据库密码。
- PostgreSQL、磁盘、连接池、HTTP 5xx、HTTP 402、计费失败和 Outbox 重试有告警。

## 回滚

代码回滚与数据库恢复分开处理：

1. 立即停止写流量并切回上一镜像。
2. 如果新 schema 与旧代码兼容，保留数据库，只回滚应用。
3. 如果不兼容，停止所有实例，恢复发布前备份，再启动上一镜像。
4. 将 `AXONHUB_BILLING_MODE` 降为 `warn` 或 `disabled`，避免异常价格或迁移状态继续阻断请求。
5. 校验钱包余额、账本交易、订单、订阅、用量记录和 Outbox，再恢复流量。

支付通知和账本写入有幂等约束，但回滚后仍应按订单号、请求 ID 和幂等键核对，不应直接修改钱包余额。
