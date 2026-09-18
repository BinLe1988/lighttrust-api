# 开发日志 - 2026-09-18

## 一、科研工作台 Bug 修复

### 403 错误修复：TokenKey 空值保护

**问题**：research 路由使用 `UserAuth()` 中间件，该中间件只设置用户信息，不设置 `token_key`。导致 `PreConsumeTokenQuota` 中 `GetTokenByKey("")` 查询失败，返回 `gorm.ErrRecordNotFound`。

**修复**：`service/quota.go` - 在 `PreConsumeTokenQuota` 中增加 TokenKey 空值保护，当 TokenKey 为空时跳过 token 级别额度预扣。

### 500 错误修复：渠道选择逻辑

**问题**：`getChannel` 函数在 `ChannelMeta == nil` 时走简化分支，从 context 获取 `channel_id`。但 authHelper 未设置该值，导致返回空渠道对象（ID=0），后续 URL 拼接失败。

**修复**：`controller/relay.go` - 在 `getChannel` 简化分支中增加判断，当 `channel_id == 0` 时走正常渠道选择流程。

### 500 错误修复：模型名为空

**问题**：`ResearchChat` 未将模型名写入 context，导致 `genBaseRelayInfo` 中 `OriginModelName` 为空，渠道选择无法匹配。

**修复**：`controller/research.go` - 在调用 Relay 前添加 `c.Set("original_model", modelName)`。

### 404 错误修复：请求路径修正

**问题**：research/chat 请求路径为 `/api/research/chat`，非标准 `/v1/chat/completions`，导致 Azure URL 拼接逻辑中 `task` 变量提取错误。

**修复**：`controller/research.go` - 在调用 Relay 前将请求路径修改为 `/v1/chat/completions`。

## 二、科研助手外部接入（Phase 1-3）

### Phase 1：OpenAI 兼容 API

**功能**：将科研工作台能力包装为 6 个特殊模型名，外部 Agent 通过标准 OpenAI API 调用即可自动获得系统提示词注入和模型路由。

**修改文件**：
- `relay/helper/model_mapped.go` - 新增 `researchStageConfig` 映射表，识别 `research-*` 前缀并自动映射到底层模型
- `relay/compatible_handler.go` - 在 `TextHelper` 中增加 research 系统提示词注入逻辑

**模型名映射**：

| 对外模型名 | 底层模型 | 用途 |
|-----------|---------|------|
| research-literature | gpt-5.2-1 | 文献调研 |
| research-experiment | gpt-5.6-luna-1 | 实验设计 |
| research-analysis | gpt-5.6-luna-1 | 数据分析 |
| research-writing | gpt-5.5-1 | 论文撰写 |
| research-coding | gpt-5.6-luna-1 | 代码实现 |
| research-review | gpt-5.5-1 | 综述写作 |

### Phase 2：MCP Server

**功能**：新增 MCP 协议端点，支持 `tools/list` 和 `tools/call` 方法，让支持 MCP 的 Agent（Claude Desktop、OpenClaw 等）直接调用科研助手。

**新增文件**：`controller/mcp.go` - MCP JSON-RPC 协议控制器

**修改文件**：`router/api-router.go` - 注册 `/api/mcp` 端点，使用 `TokenAuthReadOnly` 中间件

**可用工具**：
- `research_chat` - 科研助手对话（参数：stage, message）
- `research_stages` - 获取科研阶段列表

### Phase 3：OpenClaw Skill 包

**功能**：创建 OpenClaw 可安装的 Skill 包，包含完整的技能描述、使用指南和调用示例。

**新增文件**：`skills/research-assistant/SKILL.md`

## 三、模型定价倍率调整

**问题**：gpt-5.5-1、gpt-5.6-luna-1、gpt-5.2-1 未配置单独倍率，使用默认倍率 37.5，导致额度显示与实际成本差异巨大。

**修复**：`setting/ratio_setting/model_ratio.go` - 在 `defaultModelRatio` 中为三个模型添加倍率 1.0（1 额度 ≈ 0.01 元人民币）。

## 四、用户账户调整

- 将 token `fsbkMR4z4DmA9usEAWf8V4dCDCF8Muavz8c02rbV0tMsBqLx` 关联用户从 18822 切换为 nickluo（余额充足）
- 按新倍率比例调整 nickluo 账户余额
