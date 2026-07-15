# UI 修改记录

## 2026-07-15 — lighttrust-api 对账中心

**技术栈：** React 19 + TanStack Router/Query + Tailwind CSS 4 + i18next
**修改背景：** 为 Bedrock 三层对账插件提供配置、运行、查询和导出管理界面。

### 修改清单

| 类型 | 文件路径 | 修改内容 | 结果 |
|---|---|---|---|
| 路由 | `web/default/src/routes/_authenticated/reconciliation` | 管理员权限路由 | 已完成 |
| API/类型 | `web/default/src/features/reconciliation` | 对账 API 与类型 | 已完成 |
| 组件/交互 | `web/default/src/features/reconciliation` | 六标签工作台、配置 CRUD、权限控制 | 已完成 |
| i18n | `web/default/src/i18n/locales` | 中英文对账文案 | 已完成 |

### 技术路线

- 样式：复用工程主题 token 和现有 UI 原语。
- i18n：所有用户可见文案通过 `t()`，最后执行同步脚本。
- 组件：服务端数据由 TanStack Query 管理，表单状态保持在业务组件内部。
- 交互：配置选择驱动所有结果查询；运行、诊断、重试后统一失效相关查询。

### 参考规范

- 北电数智数据可视化大屏设计规范 V2.0
- `ui-modification` skill 的组件、状态管理和 i18n 参考

### 验证结果

- `oxlint`：新增对账目录、路由、导航与权限文件通过。
- `tsgo -b`：通过。
- `rsbuild build`：生产构建通过。
