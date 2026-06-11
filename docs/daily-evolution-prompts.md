# Fusion Git Desk 每日演化提示词

状态日期：2026-06-11

本文档用于驱动 Fusion Git Desk 的持续演化工作流：每天巡检仓库、发现需求、选择一个可落地的小目标、实现、验证，并把通过验证的调整发布到远程。

## 使用原则

- 每次只选择一个小而完整的目标，优先修复用户可感知的问题、性能瓶颈和发布链路缺口。
- 自动化可以提出需求和实现改动，但不能跳过质量门禁。
- Git 写操作必须显式、可追踪、可回滚。默认直接在 `main` 上本地提交；只有在全部验证通过后才允许推送 `main`。
- 默认发布到远程的含义是把已验证提交直接推送到 `origin/main`，触发 GitHub Actions 远程打包；只有版本号、标签和 release 条件明确时才创建 `v*` 标签触发 GitHub Release。
- 日常演化固定在 `main` 完成，实现、验证、提交、推送都不离开主干。
- 每轮工作都要更新 `WORKDOC.md`，按日期追加简短工作记录。
- 保持项目约束：Go + Wails v2 后端，Vue 3 + TypeScript + Vite 前端，Git 操作通过系统 `git`，Windows 下 Git 子进程保持隐藏窗口启动。

## 变量

复制提示词时可替换这些变量：

```text
REPO_PATH=E:\my\fusion-git-desk
DATE=YYYY-MM-DD
RELEASE_MODE=main-direct
```

`RELEASE_MODE` 建议取值：

- `main-direct`：默认模式。直接在 `main` 上提交并推送到 `origin/main`，触发远程 macOS 打包 workflow。
- `tag-release`：正式发布模式。所有验收通过后更新版本和 changelog，创建并推送 `v*` 标签。
- `local-only`：只做本地改动和报告，不推送。

日常演化统一使用 `main-direct`，直接在当前主干完成实现、验证和推送。

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
3. 目标必须足够小，可以在 `main` 上完成、验证、提交并推送。
4. 实现前写出简短计划；实现时遵循现有代码风格；避免无关重构。
5. 修改后运行可用的质量门禁，至少包括：
   - cd frontend && pnpm build
   - go test ./...
6. 如果本机缺少 Go、Node、pnpm、Wails 或远程凭据，记录阻塞原因，并尽可能完成可验证的部分。
7. 通过验证后按 RELEASE_MODE 发布：
   - main-direct：直接提交到本地 `main`，推送到 `origin/main`；如果远程凭据缺失，保留本地提交并清楚记录阻塞原因。
   - tag-release：确认版本号，更新必要版本记录，提交，推送 `main`，创建并推送 `v*` 标签。
   - local-only：只保留本地提交或改动报告，不推送。
8. 只按 `main-direct` 执行日常演化，始终在当前 `main` 完成。
9. 更新 WORKDOC.md，按日期追加简短工作记录。
10. 最终输出：今日目标、改动摘要、验证结果、发布结果、后续建议。
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
3. 更新 WORKDOC.md，按日期追加本轮简短工作记录。
4. 阅读最近提交和版本号，判断本次是普通远程发布还是正式 release。

默认发布流程：
- 确认当前位于 `main`。
- 提交信息使用：type(scope): summary。
- 直接推送到 `origin/main`。
- 推送后检查 GitHub Actions 的 macOS package workflow，确认远程打包是否已启动或完成。
- 发布流程固定使用 `main`。
- 如果当前不在 `main`，先回到 `main` 并确认工作区干净。

正式 release 流程只在明确要求 RELEASE_MODE=tag-release 时执行：
- 确认新版本号。
- 更新版本记录和必要文档。
- 推送已验证的 `main`。
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

## Codex 自动化执行提示词

```text
Automation: Fusion Git Desk 每日演化
Automation ID: fusion-git-desk
Automation memory: $CODEX_HOME/automations/fusion-git-desk/memory.md

进入 E:\my\fusion-git-desk，阅读 AGENTS.md、README.md、WORKDOC.md、docs/requirements.md、docs/roadmap.md、docs/performance.md 和 docs/daily-evolution-prompts.md。

按照 docs/daily-evolution-prompts.md 中的“总控提示词”执行一次每日演化。目标按天收敛：每天最多主动完成一个小而完整的演化目标；如果当天已有未完成目标，则继续推进同一个目标；如果当天已有完成并推送的目标，且没有更严重的构建、测试、发布或 P0/P1 问题，则只做简短巡检报告，不要为了重复触发而制造新改动。

本次使用：
- DATE=今天日期
- RELEASE_MODE=main-direct

所有实现都在当前本地 `main` 上完成；不要创建功能分支、远程分支、worktree 或 PR。验证通过后提交到 `main`，直接推送 `origin/main`，由 GitHub Actions 触发远程 macOS 打包 workflow。不要在未明确 `tag-release` 模式时创建 `v*` 标签。

执行要求：
1. 先检查 `git status --short --branch`、最近提交、WORKDOC.md 今日记录和 GitHub Actions/发布状态。
2. 优先从 docs/roadmap.md 和 docs/performance.md 中选择事项；如果发现更严重的构建、测试或发布问题，则优先修复。
3. 实现前给出简短计划；修改时遵循现有代码风格，避免无关重构。
4. 修改后更新 WORKDOC.md，按日期追加本轮简短记录。
5. 修改后运行可用质量门禁，至少尝试：
   - cd frontend && pnpm build
   - go test ./...
6. 如果涉及桌面打包或发布链路，额外尝试 `wails build`；如果当前平台无法生成目标平台产物，说明限制和替代产物。
7. 验证通过后提交并直接推送 `origin/main`；推送后检查 GitHub Actions 的 macOS package workflow 是否启动或完成。
8. 不创建日常 PR。即使 gh CLI 可用且已登录，也只用于查询 workflow 或发布状态。
9. 如果缺少工具、凭据、网络或远程 API 受限，记录阻塞原因，并尽可能完成本地可验证部分。

完成后必须输出：
- 今日选择的目标和原因
- 实现摘要
- 运行过的验证命令和结果
- WORKDOC.md 是否已更新
- 是否已提交、是否已推送 `origin/main`、是否创建 PR（通常为否）、远程 workflow 状态
- 下一次建议处理的事项
```

## 建议节奏

- 每日：巡检 + 一个小改进 + 主干提交。
- 每周：整理 roadmap，关闭过期需求，挑一个 P1 产品能力。
- 每个正式版本：只在所有门禁通过后推送 `v*` 标签，让 GitHub Actions 生成 macOS Release artifact。
- 每月：做一次性能审计，重点看扫描耗时、diff 渲染和后台任务拥堵。
