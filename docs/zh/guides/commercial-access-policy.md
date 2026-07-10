# 商业权限与归属策略

本文定义商业化 fork 的管理员权限、资源归属和停用策略。

## 导航与授权层级

| 层级 | 功能 | 授权来源 |
| --- | --- | --- |
| 消费者工作空间 | Home、工作空间、个人 API Key、Playground、本人 Usage、模型目录、钱包、Profile | Active 用户和 Active Project membership |
| Project administration | 共享 Key、Prompts、项目请求、Trace、Thread、成员、角色 | Project owner、直接 Project scope 或 Project role scope |
| System administration | Projects、Channels、上游账户、系统模型、用户、角色、计费管理、系统设置 | 系统 Owner 或明确的系统 scope |

消费者能力在请求时根据 membership 派生。不要为了让历史成员调用 AI，而给旧 membership 回填系统管理 scope。

## 资源归属

- BillingAccount 和钱包属于用户。
- Project 负责资源隔离和价格上下文，不持有用户资金。
- 个人 API Key 属于一个用户，并限定在一个 Project 内。
- Service account 属于 Project 运维凭据，创建者只用于审计。
- 用量扣费必须保留用户、Project、API Key、价格快照、BillingRecord 和 LedgerTransaction 关联。

## 停用策略

停用用户会使密码登录、JWT，以及该用户拥有的 `personal` 和历史 `user` 类型 API Key 失效。Project `service_account` 不会因为创建者停用而自动失效，成员离职时管理员必须单独审查共享 Service Account。

归档 Project 会使该 Project 下所有 API Key 无法认证。单独禁用或归档某个 API Key 只影响该凭据。

## 升级与回滚策略

升级前：

1. 停止写入并创建当前版本 PostgreSQL dump。
2. 记录镜像摘要、Git tag、schema baseline 和支付加密根密钥位置。
3. 将 dump 恢复到隔离数据库并运行商业角色矩阵。
4. 只启动一个可执行迁移的新版本实例，迁移完成后再扩容。

发布 manifest 必须保持用户、Project ID、membership、API Key 标识、价格规则、钱包、账本、订单、订阅、Request、UsageLog、UsageBillingRecord 及其关联不变。只有旧程序明确兼容新 schema 时才可以只回滚应用；否则必须恢复已验证的发布前备份。

完整自动化门禁命令：

```bash
./scripts/e2e/commercial-release-gate.sh --full
```
