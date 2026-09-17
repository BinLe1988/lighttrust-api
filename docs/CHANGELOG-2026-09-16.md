# 开发日志 - 2026-09-16

## 一、科研工作台（Research Workbench）

### 功能概述
新增垂直场景工作入口，首个场景为科研场景。根据科研的不同阶段（文献调研、实验设计、数据分析、论文撰写、代码实现、综述写作）自动选择最优模型，减少 token 浪费。

### 后端实现

**新增文件**：`controller/research.go`
- 定义 6 个科研阶段，每个阶段配置专属系统提示词和路由策略
- 提供 3 个 API 接口：
  - `GET /api/research/stages` - 获取阶段列表
  - `GET /api/research/stages/:stage` - 获取单个阶段配置
  - `POST /api/research/chat` - 科研场景对话（支持 SSE 流式）

**修改文件**：`router/api-router.go`
- 注册 `/api/research` 路由组，使用 UserAuth 中间件

### 前端实现

**classic 主题**：
- 新增 `web/classic/src/pages/Research/index.jsx` - 科研工作台页面
- 修改 `web/classic/src/App.jsx` - 注册 `/console/research` 路由
- 修改 `web/classic/src/components/layout/SiderBar.jsx` - 添加菜单入口

**default 主题**：
- 新增 `web/default/src/routes/_authenticated/research/index.tsx` - 路由页面
- 新增 `web/default/src/features/research/index.tsx` - 功能组件
- 修改 `web/default/src/hooks/use-sidebar-data.ts` - 侧边栏添加入口

### 科研阶段与模型策略

| 阶段 | 策略 | 说明 |
|---|---|---|
| 文献调研 | omni-auto:balanced | 强检索和总结能力 |
| 实验设计 | omni-auto:coding | 逻辑推理和结构化输出 |
| 数据分析 | omni-auto:coding | 数据处理和代码生成 |
| 论文撰写 | omni-auto:balanced | 高质量文本生成 |
| 代码实现 | omni-auto:coding | 代码生成能力最强 |
| 综述写作 | omni-auto:balanced | 长文本理解和总结 |

## 二、Azure 渠道配置

### 新增 Azure 渠道
- 渠道 ID：6，名称：azure-gpt5
- 类型：OpenAI 兼容（type=1）
- Base URL：`https://mcygui522539-5283-resource.services.ai.azure.com/openai`
- 模型：gpt-5.5-1、gpt-5.2-1、gpt-5.6-luna-1、gpt-image-2

### API 版本按模型区分

**修改文件**：`dto/channel_settings.go`
- 新增 `AzureModelVersions` 字段（`map[string]string`）

**修改文件**：`relay/channel/openai/adaptor.go`
- 请求构建时按模型名查找对应 API 版本

**模型版本映射**：
| 模型 | API 版本 |
|---|---|
| gpt-5.5-1 | 2026-04-24 |
| gpt-5.2-1 | 2025-12-11 |
| gpt-5.6-luna-1 | 2026-07-09 |
| gpt-image-2 | 2026-04-21 |

## 三、其他修复

- 修复 classic 主题侧边栏科研工作台入口不可见问题（itemKey 过滤）
- 修复 default 主题构建缓存导致代码修改不生效问题
- 修复科研工作台 API 调用方式（getApiUrl → api 实例）

## 四、文件变更清单

| 文件 | 状态 | 说明 |
|---|---|---|
| `controller/research.go` | 新增 | 科研场景控制器 |
| `router/api-router.go` | 修改 | 注册科研场景路由 |
| `dto/channel_settings.go` | 修改 | 新增 AzureModelVersions 字段 |
| `relay/channel/openai/adaptor.go` | 修改 | Azure API 版本按模型区分 |
| `web/classic/src/pages/Research/index.jsx` | 新增 | classic 科研工作台页面 |
| `web/classic/src/App.jsx` | 修改 | 注册科研工作台路由 |
| `web/classic/src/components/layout/SiderBar.jsx` | 修改 | 添加菜单入口 |
| `web/default/src/routes/_authenticated/research/index.tsx` | 新增 | default 路由页面 |
| `web/default/src/features/research/index.tsx` | 新增 | default 功能组件 |
| `web/default/src/hooks/use-sidebar-data.ts` | 修改 | 侧边栏添加入口 |
