---
name: research-assistant
description: "科研助手技能，支持文献调研、实验设计、数据分析、论文撰写、代码实现、综述写作。触发词：科研、论文、文献、实验、数据分析、综述、research、paper、literature、experiment。通过 OpenAI 兼容 API 或 MCP 协议调用，自动选择最优模型和注入专业系统提示词。"
---

# 科研助手 (Research Assistant)

一站式科研工作台，覆盖学术研究全流程的 AI 助手。通过预配置的模型路由和系统提示词，为每个科研阶段提供专业化的 AI 支持。

## 快速导航

```
用户需求
├── 文献调研 / 找论文 / 读文献 → research-literature (gpt-5.2-1)
├── 实验设计 / 设计实验方案 → research-experiment (gpt-5.6-luna-1)
├── 数据分析 / 处理数据 / 统计 → research-analysis (gpt-5.6-luna-1)
├── 论文撰写 / 写论文 / 润色 → research-writing (gpt-5.5-1)
├── 代码实现 / 写代码 / 算法 → research-coding (gpt-5.6-luna-1)
└── 综述写作 / 写综述 / 文献综述 → research-review (gpt-5.5-1)
```

## AI 使用指南

### 场景判断

当用户提出科研相关需求时，根据需求类型选择对应的模型名：

| 用户需求关键词 | 模型名 | 底层模型 | 用途 |
|---------------|--------|---------|------|
| 文献、论文检索、读论文、文献调研 | `research-literature` | gpt-5.2-1 | 检索和总结学术文献 |
| 实验、实验方案、变量设计 | `research-experiment` | gpt-5.6-luna-1 | 设计实验方案 |
| 数据分析、统计、可视化、处理数据 | `research-analysis` | gpt-5.6-luna-1 | 数据处理和代码生成 |
| 写论文、润色、摘要、引言 | `research-writing` | gpt-5.5-1 | 学术写作和润色 |
| 写代码、算法实现、编程 | `research-coding` | gpt-5.6-luna-1 | 科研编程 |
| 综述、文献综述、研究现状 | `research-review` | gpt-5.5-1 | 综述写作 |

### 调用方式

#### 方式一：OpenAI 兼容 API（推荐）

直接使用标准 OpenAI API 格式调用，指定模型名即可自动获得专业系统提示词：

```python
import openai
client = openai.OpenAI(
    base_url="https://niul-ai.cn/v1",
    api_key="sk-your-key"
)
response = client.chat.completions.create(
    model="research-literature",  # 根据需求选择模型名
    messages=[{"role": "user", "content": "用户的问题"}]
)
```

```bash
curl https://niul-ai.cn/v1/chat/completions \
  -H "Authorization: Bearer sk-your-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"research-literature","messages":[{"role":"user","content":"Transformer 最新进展"}]}'
```

#### 方式二：MCP 协议

通过 MCP 协议调用，支持 tools/list 和 tools/call：

```json
// MCP 端点
POST https://niul-ai.cn/api/mcp
Authorization: Bearer sk-your-key

// tools/call 示例
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "research_chat",
    "arguments": {
      "stage": "literature",
      "message": "Transformer 最新进展"
    }
  }
}
```

## 操作流程

### 文献调研

1. 识别用户需求为文献调研
2. 使用模型名 `research-literature`
3. 系统自动注入文献调研专家提示词
4. 返回结构化的文献分析结果

### 实验设计

1. 识别用户需求为实验设计
2. 使用模型名 `research-experiment`
3. 系统自动注入实验设计专家提示词
4. 返回包含假设、方法、预期结果的实验方案

### 数据分析

1. 识别用户需求为数据分析
2. 使用模型名 `research-analysis`
3. 系统自动注入数据分析专家提示词
4. 返回可执行的代码和分析结果

### 论文撰写

1. 识别用户需求为论文撰写
2. 使用模型名 `research-writing`
3. 系统自动注入学术写作专家提示词
4. 返回符合学术规范的文本

### 代码实现

1. 识别用户需求为代码实现
2. 使用模型名 `research-coding`
3. 系统自动注入科研编程专家提示词
4. 返回完整可运行的代码

### 综述写作

1. 识别用户需求为综述写作
2. 使用模型名 `research-review`
3. 系统自动注入综述写作专家提示词
4. 返回系统性的综述内容

## 交互示例

### 示例 1：文献调研

**用户**："帮我调研 Transformer 模型的最新进展"

**AI 操作**：
1. 识别为文献调研需求
2. 调用 `research-literature` 模型
3. 系统自动注入文献调研系统提示词
4. 返回结构化的文献分析

### 示例 2：实验设计

**用户**："我想设计一个对比实验，验证新算法的效果"

**AI 操作**：
1. 识别为实验设计需求
2. 调用 `research-experiment` 模型
3. 系统自动注入实验设计系统提示词
4. 返回包含假设、变量、方法的实验方案

### 示例 3：数据分析

**用户**："帮我分析这组实验数据，用 Python 做 t 检验"

**AI 操作**：
1. 识别为数据分析需求
2. 调用 `research-analysis` 模型
3. 系统自动注入数据分析系统提示词
4. 返回可执行的 Python 代码和统计结果解释

## 技术架构

```
外部 Agent (OpenClaw / Claude Desktop / DeepSeek Plugin)
    │
    ├── OpenAI 兼容 API: POST /v1/chat/completions
    │   model: research-{stage}
    │
    └── MCP 协议: POST /api/mcp
        method: tools/call
        params: { name: "research_chat", arguments: { stage, message } }
    │
    ▼
new-api (niul-ai.cn:3000)
    │
    ├── 识别 research-* 模型名
    ├── 注入对应系统提示词
    ├── 路由到 Azure 渠道 (gpt-5.2-1 / gpt-5.5-1 / gpt-5.6-luna-1)
    └── 返回标准 OpenAI 响应
```

## 故障排查

| 问题 | 可能原因 | 解决方案 |
|------|---------|---------|
| 401 Unauthorized | API Key 无效 | 检查 Authorization header 格式 |
| 403 Forbidden | 额度不足 | 检查账户余额 |
| 404 Not Found | 模型名错误 | 确认使用正确的 research-* 模型名 |
| 500 Internal Error | 渠道配置问题 | 联系管理员检查 Azure 渠道状态 |
