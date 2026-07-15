# 对账中心 UI 修改方案

## 技术栈

- 框架：React 19 + TypeScript
- 路由/服务端状态：TanStack Router + TanStack Query
- 样式：Tailwind CSS 4 + 工程现有主题 token
- 本地化：i18next / react-i18next

## 修改清单

| # | 类型 | 位置 | 修改内容 | 技术路线 |
|---|---|---|---|---|
| 1 | 路由/权限 | `src/routes/_authenticated/reconciliation` | 新增管理员对账中心路由 | 复用 `hasPermission` 和 403 跳转 |
| 2 | 导航 | `src/hooks/use-sidebar-data.ts` | 管理区新增对账中心入口 | 复用现有侧栏数据结构 |
| 3 | API/类型 | `src/features/reconciliation` | 配置、诊断、运行、三层结果、导出 | Axios + TanStack Query |
| 4 | 组件/交互 | `src/features/reconciliation` | 六标签工作台、配置表单、分页表格 | 工程现有 Card/Tabs/Table/Input/Button |
| 5 | i18n | `src/i18n/locales` | 同步新增用户可见文案 | `i18n:sync` |

## 样式规范对照

- 颜色：只使用工程语义 token，如 `bg-card`、`text-muted-foreground`、状态 Badge。
- 字体：标题 24px，正文 14px，标识符和时间使用等宽字体。
- 间距：页面、卡片和控件采用 4px 网格现有 Tailwind 间距。
- 布局：指标卡在断点下完整填满行，不制造半行空白。

## 风险点

- 结果量大时仅允许后端限制的 10,000 行同步导出。
- External ID 永不回显；更新时留空表示保留原值。
- 定时任务默认关闭，完成诊断和手工回填后再通过环境变量启用。
