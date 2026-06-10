# Fusion Git Desk 每日演化提示词

状态日期：2026-06-10

本文档用于驱动 Fusion Git Desk 的持续演化工作流：每天巡检仓库、发现需求、选择一个可落地的小目标、实现、验证，并把通过验证的调整发布到远程。

## 使用原则

- 每次只选择一个小而完整的目标，优先修复用户可感知的问题、性能瓶颈和发布链路缺口。
- 自动化可以提出需求和实现改动，但不能跳过质量门禁。
- Git 写操作必须显式、可追踪、可回滚。日常演化默认从 `main` 出发，在 `BRANCH=codex/daily-evolution-DATE` 上提交并推送远程分支。
- 默认发布到远程的含义是把已验证提交推送到日常演化分支，并在可用时创建 PR；不要直接推送 `main`。
- 只有版本号、标签和 release 条件明确，且 `RELEASE_MODE=tag-release` 时，才创建 `v*` 标签触发 GitHub Release。
- 保持项目约束：Go + Wails v2 后端，Vue 3 + TypeScript + Vite 前端，Git 操作通过系统 `git`，Windows 下 Git 子进程保持隐藏窗口启动。

## 变量

复制提示词时可替换这些变量：

```text
REPO_PATH=E:\my\fusion-git-desk
DATE=YYYY-MM-DD
BRANCH=codex/daily-evolution-YYYY-MM-DD
RELEASE_MODE=branch-pr
```

`RELEASE_MODE` 建议取值：

- `branch-pr`：默认模式。从 `main` 出发提交到 `BRANCH`，推送远程分支，并在 gh CLI 可用且已登录时创建 PR。
- `main-direct`：维护者手动要求时使用。直接在 `main` 上提交并推送到 `origin/main`。
- `tag-release`：正式发布模式。所有验收通过后更新版本和 changelog，创建并推送 `v*` 标签。
- `local-only`：只做本地改动和报告，不推送。

日常演化应传入 `BRANCH=codex/daily-evolution-DATE` 和 `RELEASE_MODE=branch-pr`。不要在未明确 `tag-release` 模式时创建 `v*` 标签。

## 总控提示词

```text
你是 Fusion Git Desk 的持续演化代理。请在 REPO_PATH 仓库中工作，目标是在今天完成一个小而完整、可以验证、可以发布到远程的改进。

项目背景：
- Fusion Git Desk 是 Wails v2 桌面应用，用于管理多 Git 仓库工作区。
- 后端是 Go package main。
- 前端是 Vue 3 + TypeScript + Vite。
- Git 集成通过系统 git 和 os/exec 完成。
- 自动 pull 必须只作用于干净仓库，除非用户明确选择其他策略。
- Windows 下 Git 子进程必须隐藏窗口启动。

工作要求：
1. 先读取 AGENTS.md、README.md、docs/requirements.md、docs/roadmap.md、docs/performance.md 和当前 git 状态。
2. 从产品价值、性能、可靠性、测试覆盖和发布链路中选择一个最值得今天处理的目标。
3. 目标必须足够小，可以基于 `main` 完成、验证、提交到日常演化分支并推送。
4. 实现前写出简短计划；实现时遵循现有代码风格；避免无关重构。
5. 修改后运行可用的质量门禁，至少包括：
   - cd frontend && pnpm build
   - go test ./...
6. 如果本机缺少 Go、Node、pnpm、Wails 或远程凭据，记录阻塞原因，并尽可能完成可验证的部分。
7. 通过验证后按 RELEASE_MODE 发布：
   - branch-pr：提交到本地 `BRANCH` 或从当前 `main` HEAD 推送到 `origin/BRANCH`；如果 gh CLI 可用且已登录则创建 PR；如果远程凭据缺失，保留本地提交并清楚记录阻塞原因。
   - main-direct：仅在明确要求时直接提交到本地 `main`，推送到 `origin/main`。
   - tag-release：确认版本号，更新必要版本记录，提交，推送 `main`，创建并推送 `v*` 标签。
   - local-only：只保留本地提交或改动报告，不推送。
8. 不要直接推送 `main`，不要在未明确 `tag-release` 模式时创建 `v*` 标签。
9. 最终输出：今日目标、改动摘要、验证结果、提交/推送/PR 结果、后续建议。
```

## 每日巡检提示词

```text
请对 Fusion Git Desk 做一次每日巡检，只输出高信号结果，不要开始实现。

请检查：
1. 仓库状态：当前分支、未提交改动、最近提交、远程跟踪关系。
2. 文档状态：README、requirements、roadmap、performance 是否与代码现状冲突。
3. 前端风险：大型列表/diff 渲染、异步竞态、状态派生、构建错误。
4. 后端风险：Git 命令 timeout、Windows hidden-window startup、路径边界、未跟踪文件、符号链接、安全 pull。
5. 测试风险：已有测试覆盖是否足以保护最近功能。
6. 发布风险：GitHub Actions、版本号、release artifact、跨平台打包限制。

请按 P0/P1/P2 列出最多 7 个候选事项。每个事项包含：
- 问题
- 用户价值
- 涉及文件
- 预估风险
- 建议验收方式

最后推荐今天最适合自动完成的 1 个事项。
```

## 需求提出提示词

```text
请基于 Fusion Git Desk 的产品定位，为下一轮演化提出可执行需求。

约束：
- 不要提出泛泛的大功能。
- 每个需求必须能落到具体界面、后端能力或发布流程。
- 优先考虑多仓库管理、Git 操作安全、性能、错误可观测和发布自动化。
- 每个需求应能被单独实现和验证。

输出格式：
1. 需求名称
2. 用户场景
3. MVP 范围
4. 非目标
5. 涉及模块
6. 验收标准
7. 风险与回滚策略
```

## 实现提示词

```text
请实现以下已选需求：<填写需求名称和验收标准>。

执行规则：
- 先阅读相关文件，不要凭空改代码。
- 后端优先使用结构化 Git 输出，例如 git status --porcelain。
- 所有长耗时 Git 命令必须有 timeout。
- Windows Git 子进程必须继续使用隐藏窗口启动。
- 前端遵循现有 Vue 组件和 CSS 风格，不引入不必要依赖。
- 性能相关改动必须说明避免了哪些重复计算、阻塞 I/O 或过量 DOM。
- 不要改动无关格式、锁文件或发布产物，除非本需求需要。

完成后运行：
- cd frontend && pnpm build
- go test ./...

如果验证失败，先修复；如果环境缺失导致无法运行，说明缺失项和可替代验证。
```

## 性能优化提示词

```text
请对 Fusion Git Desk 做一次性能导向的小步优化。

重点观察：
- 多仓库扫描时每个仓库的 Git 命令数量。
- git status、diff、branch、remote、log 是否可以减少调用或延迟加载。
- 未跟踪目录、大 diff、大量变更文件是否会造成磁盘读取或 DOM 爆炸。
- 前端 computed/watch 是否有重复遍历、竞态覆盖或高频后端写入。
- 自动刷新、自动 fetch、自动 pull 是否可能重叠执行。

请选择一个风险可控的优化点完成实现，并在 docs/performance.md 中追加记录：
- 优化前的问题
- 采取的策略
- 性能取舍
- 验收方式
```

## 发布提示词

```text
请把本轮已验证改动发布到远程。

发布前检查：
1. git status --porcelain，确认只有本轮相关改动。
2. 运行或确认已经运行：
   - cd frontend && pnpm build
   - go test ./...
3. 阅读最近提交和版本号，判断本次是普通远程发布还是正式 release。

默认发布流程：
- 确认当前位于 `main`。
- 提交信息使用：type(scope): summary。
- 使用 `BRANCH=codex/daily-evolution-DATE`。
- 推送到 `origin/BRANCH`。
- 如果 gh CLI 可用且已登录，创建从 `BRANCH` 到 `main` 的 PR。
- 不直接推送 `main`。

正式 release 流程只在明确要求 RELEASE_MODE=tag-release 时执行：
- 确认新版本号。
- 更新版本记录和必要文档。
- 合并或推送到 release 目标分支。
- 创建 v* 标签并推送。
- 等待 GitHub Actions 生成 Release artifact，报告 workflow 结果。

不要在未通过验证时推送 release 标签。
```

## 故障处理提示词

```text
本轮自动演化失败了。请做恢复分析，不要继续扩大改动。

请输出：
1. 当前 git 状态。
2. 已修改文件列表，以及哪些是本轮产生的。
3. 失败发生在哪一步：读取、实现、构建、测试、提交、推送、CI、release。
4. 最小修复建议。
5. 是否需要回滚本轮改动。
6. 如果需要回滚，只给出候选命令，不要自动执行 destructive 命令，除非用户明确授权。
```

## 推荐每日定时任务提示词

```text
进入 E:\my\fusion-git-desk，使用 docs/daily-evolution-prompts.md 中的“总控提示词”执行一次每日演化。

本次使用：
- DATE=今天日期
- BRANCH=codex/daily-evolution-今天日期
- RELEASE_MODE=branch-pr

不要直接推送 `main`，不要在未明确 `tag-release` 模式时创建 `v*` 标签。

请每天只完成一个小目标。优先从 docs/roadmap.md 和 docs/performance.md 中选择事项；如果发现更严重的构建、测试或发布问题，则优先修复。

完成后必须输出：
- 今日选择的目标和原因
- 实现摘要
- 运行过的验证命令和结果
- 是否已提交、是否已推送远程分支、是否已创建 PR
- 下一次建议处理的事项
```

## 建议节奏

- 每日：巡检 + 一个小改进 + 远程分支 PR。
- 每周：整理 roadmap，关闭过期需求，挑一个 P1 产品能力。
- 每个正式版本：只在所有门禁通过后推送 `v*` 标签，让 GitHub Actions 生成 macOS Release artifact。
- 每月：做一次性能审计，重点看扫描耗时、diff 渲染和后台任务拥堵。
