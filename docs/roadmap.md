# Fusion Git Desk 下一步计划与优化清单

状态日期：2026-06-05

本次发布版本：`v0.1.1`

## 本次目标

本轮 `/goal` 聚焦可以直接落地、可以验证、可以发布的 P0 项：

- [x] 输出下一步产品、工程和性能优化 Markdown 文档。
- [x] 完善 GitHub Actions：推送 `main` 保持构建 artifact，推送 `v*` 标签时创建 GitHub Release 并上传 macOS 包。
- [x] 优化前端设置保存节奏，避免每次设置变化都立即写入后端。
- [x] 优化前端 diff/branch 加载竞态，避免快速切换仓库或文件时旧请求覆盖新状态。
- [x] 加强未跟踪文件 diff 预览的路径边界和符号链接处理。
- [x] 补充关键单元测试覆盖路径、符号链接和带空格文件 diff 解析。

## 下一步产品事项

### P1：多仓库管理体验

- 仓库收藏、分组和标签：支持按客户、服务域、项目组过滤。
- 仓库健康视图：集中展示落后、领先、冲突、无 upstream、最近 fetch 失败等状态。（初版已支持仓库列表高优先排序、顶部指标和选中仓库状态建议。）
- 工作区 manifest 导入导出：让团队共享一组仓库清单和推荐扫描深度。

### P1：Git 操作闭环

- 文件级 stage/unstage：先限制在单文件和显式确认操作。
- Commit 草稿：提供消息输入、待提交文件预览和提交后刷新。
- Push：默认只对当前仓库执行，显示 upstream 和 ahead 数量，避免批量误推。

### P2：桌面集成

- 系统托盘：支持后台刷新和状态提醒。
- 后台计划任务：与自动 fetch/pull 策略结合，提供可暂停状态。
- 通知策略：只对失败、冲突、落后数量变化等高价值事件提醒。

## 工程优化事项

- 发布流程：标签发布自动生成 GitHub Release，后续可继续加入 Windows 构建产物。
- 质量门禁：保持 `pnpm build`、`go test ./...`、macOS 打包脚本在 CI 中串联执行。
- 配置写入：前端做短 debounce，后端继续做 Normalize，避免高频写文件。
- Git 命令边界：所有长耗时 Git 命令继续使用 timeout；Windows 子进程保持 hidden-window startup。
- 错误可观测：保留 Git 原始 stderr，同时给常见场景补充用户可理解的说明。

## 性能优化事项

### 已纳入本轮

- 设置保存 debounce：减少自动刷新、数值输入和开关切换时的重复后端调用。
- stale response guard：diff 和分支请求增加序号校验，避免旧请求造成额外渲染和错误状态回写。
- 未跟踪文件预览边界：限制为仓库内普通文本文件，符号链接只给说明，不跟随读取。

### 后续优化候选

- 大型 diff 虚拟滚动：当前 diff 已限制字节数，但 DOM 行数仍可能较多；后续应按文件和行虚拟化。
- 扫描缓存：缓存上次扫描路径和目录 mtime，对未变化目录减少 `git status` 调用。
- Git 命令批处理：评估将 branch、remote、last commit 信息合并到更少 Git 调用中，降低多仓库扫描耗时。
- 并发策略可配置：当前扫描和更新使用固定上限，后续可按 CPU、磁盘类型和仓库数量动态调节。
- 前端列表虚拟化：当仓库数量超过 100 时，对仓库列表和变更文件列表启用虚拟列表。

## 发布与验收

- 本地前端验收：`cd frontend && pnpm build`
- 本地 Go 验收：`go test ./...`
- CI 验收：推送 `main` 后触发 macOS package workflow。
- Release 验收：推送 `v*` 标签后创建 GitHub Release，包含 `.zip` 和 `.dmg` 产物。

## 风险记录

- 当前本机未检测到 Go 命令，Go 单元测试需要在已安装 Go 的环境或 GitHub Actions 中验证。
- macOS 包无法在 Windows 本机交叉构建，必须依赖 macOS runner 或 Mac 机器。
- GitHub Release 创建依赖仓库 Actions 的 `GITHUB_TOKEN` 写权限，本轮已在 workflow 中声明 `contents: write`。
