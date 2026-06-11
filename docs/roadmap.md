# Fusion Git Desk 下一步计划与优化清单

状态日期：2026-06-11

本次发布版本：`v0.1.1`

## 当前阶段

当前阶段是基础能力打磨期。目标不是把功能面铺满，而是让多仓库工作区的核心闭环稳定、可解释、可验证：

1. 扫描多仓库并看清状态。
2. 识别风险并安全执行 fetch/pull。
3. 查看仓库级和文件级 diff。
4. 对当前仓库完成明确的 stage/unstage/commit。
5. 通过本地构建、Go 测试和发布链路检查证明改动可信。

如果基础闭环出现 P0/P1 缺口，优先修复基础能力。只有基础闭环健康时，才推进分组、托盘、深层缓存、复杂发布等更深入事项。

## 选题闸门

每轮只选一个目标。选题前必须回答：

- 这个问题是否影响用户看清状态、检查 diff、安全同步或小步提交？
- 是否有明确复现路径、用户价值和验收方式？
- 是否能在一次小改动内完成，不扩大无关模块？
- 是否比当前基础闭环里的其他缺口更重要？

不满足这些条件的事项先记录，不进入本轮实现。

## 已完成基础事项

本轮 `/goal` 聚焦可以直接落地、可以验证、可以发布的 P0 项：

- [x] 输出下一步产品、工程和性能优化 Markdown 文档。
- [x] 完善 GitHub Actions：推送 `main` 保持构建 artifact，推送 `v*` 标签时创建 GitHub Release 并上传 macOS 包。
- [x] 增加本地自动部署脚本：在 Windows 本机串联前端构建、Go 测试和 Wails 打包，并可通过本地 Git hook 在 `main` 提交或更新后触发。
- [x] 优化前端设置保存节奏，避免每次设置变化都立即写入后端。
- [x] 优化前端 diff/branch 加载竞态，避免快速切换仓库或文件时旧请求覆盖新状态。
- [x] 加强未跟踪文件 diff 预览的路径边界和符号链接处理。
- [x] 补充关键单元测试覆盖路径、符号链接和带空格文件 diff 解析。

## 下一步产品事项

### P0/P1：基础闭环补强

- 验证真实工作区的首次扫描体验：空目录、无权限目录、嵌套仓库、大量仓库和 Git 命令失败时都应有清楚反馈。
- 校准仓库状态解释：冲突、无 upstream、ahead、behind、本地改动、staged 文件数和 pull 保护原因应在界面中一致呈现。
- 完善 diff 可用性：普通文本、未跟踪文件、二进制文件、过大文件和无权限文件都应有稳定展示或明确说明。
- 强化小步提交闭环：stage/unstage/commit 的失败信息、按钮禁用原因和刷新结果应足够明确。
- 增加基础手动验收脚本或清单：覆盖扫描、fetch、pull 保护、diff、stage/unstage、commit 和发布前检查。

### P1：多仓库管理体验

- 仓库收藏、分组和标签：支持按客户、服务域、项目组过滤。
- 仓库健康视图：集中展示落后、领先、冲突、无 upstream、最近 fetch 失败等状态。（初版已支持仓库列表高优先排序、顶部指标和选中仓库状态建议。）
- 工作区 manifest 导入导出：让团队共享一组仓库清单和推荐扫描深度。

### P1：Git 操作闭环

- 文件级 stage/unstage：先限制在单文件和显式操作。（初版已支持选中文件 Stage/Unstage，后端校验仓库内路径并刷新 diff。）
- Commit 草稿：提供消息输入、待提交文件预览和提交后刷新。（初版已支持当前仓库已暂存文件提交，提交后刷新状态并切到 HEAD diff。）
- Push：默认只对当前仓库执行，显示 upstream 和 ahead 数量，避免批量误推。

### P2：桌面集成

- 系统托盘：支持后台刷新和状态提醒。
- 后台计划任务：与自动 fetch/pull 策略结合，提供可暂停状态。
- 通知策略：只对失败、冲突、落后数量变化等高价值事件提醒。

## 工程优化事项

- 发布流程：标签发布自动生成 GitHub Release，后续可继续加入 Windows 构建产物。
- 本地部署流程：保持 `scripts/deploy-local.ps1` 可一键生成 Windows 桌面产物，hook 只在 `main` 上触发。
- 质量门禁：保持 `pnpm build`、`go test ./...`、macOS 打包脚本在 CI 中串联执行。
- 配置写入：前端做短 debounce，后端继续做 Normalize，避免高频写文件。
- Git 命令边界：所有长耗时 Git 命令继续使用 timeout；Windows 子进程保持 hidden-window startup。
- 错误可观测：保留 Git 原始 stderr，同时给常见场景补充用户可理解的说明。

## 性能优化事项

性能优化必须先证明问题存在：有用户可感知卡顿、真实数据规模、扫描耗时、DOM 数量、命令次数或重复请求证据。没有证据时，先做观测和小范围限流，不做深层架构改造。

### 已纳入本轮

- 设置保存 debounce：减少自动刷新、数值输入和开关切换时的重复后端调用。
- stale response guard：diff 和分支请求增加序号校验，避免旧请求造成额外渲染和错误状态回写。
- 未跟踪文件预览边界：限制为仓库内普通文本文件，符号链接只给说明，不跟随读取。

### 后续优化候选

- 大型 diff 虚拟滚动：只有在真实仓库仍出现明显 diff 卡顿，且现有字节数/行数上限不能接受时再推进。
- 扫描缓存：只有在扫描耗时指标显示重复 `git status` 是主要瓶颈时再推进。
- Git 命令批处理：只有在多仓库扫描中 branch、remote、last commit 明确占用主要耗时时再推进。
- 并发策略可配置：只有在不同机器或磁盘类型上出现稳定差异时再推进。
- 仓库列表虚拟化：只有当仓库数量超过 100 且渲染成为瓶颈时再推进。

## 发布与验收

- 本地前端验收：`cd frontend && pnpm build`
- 本地 Go 验收：`go test ./...`
- 本地部署验收：运行 `powershell -ExecutionPolicy Bypass -File scripts/deploy-local.ps1` 生成 `build/bin/FusionGitDesk.exe`。
- CI 验收：推送 `main` 后触发 macOS package workflow；普通远程分支不触发自动打包。
- Release 验收：推送 `v*` 标签后创建 GitHub Release，包含 `.zip` 和 `.dmg` 产物。

## 基础手动验收清单

每次触及核心能力时，至少按影响范围选择对应检查：

- 扫描：选择包含多个 Git 仓库的工作区，确认仓库数、分支、远端和本地改动展示正确。
- diff：检查 working、staged、HEAD 视图，以及普通文本、未跟踪文本和二进制文件的展示。
- fetch/pull：确认 fetch 不改工作区；脏仓库 pull 被保护性跳过；clean 仓库只执行 `pull --ff-only`。
- stage/commit：只对当前仓库和选中文件生效；失败时有明确原因；成功后状态和 diff 刷新。
- 发布：推送 `main` 后检查 macOS package workflow 是否启动；只有明确 release 时才推送 `v*` 标签。

## 风险记录

- 当前 Windows 本机可运行 Go、pnpm 和 Wails，本地部署可以生成 Windows 产物。
- macOS 包无法在 Windows 本机交叉构建，必须依赖 macOS runner 或 Mac 机器。
- GitHub Release 创建依赖仓库 Actions 的 `GITHUB_TOKEN` 写权限，本轮已在 workflow 中声明 `contents: write`。
