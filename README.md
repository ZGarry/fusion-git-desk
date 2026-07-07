# Fusion Git Desk

一个面向多仓库工作区的跨平台 Git 桌面工作台。适合微服务、前后端分仓、客户定制分支、外包交付仓库很多的团队，把“今天哪些仓库变了、哪些落后了、哪些可以安全拉取”集中到一个清爽窗口里。

Fusion Git Desk 使用 Go + Wails 构建，目标平台为 Windows 和 macOS。

## 核心定位

Fusion Git Desk 是一个轻量的多仓库 Git 运维台。它的核心作用不是替代 IDE 或命令行，而是把一个工作区下很多仓库的状态、风险和同步动作集中起来，让用户先看清全局，再安全地处理单个仓库。

项目当前最重要的目标是把基础闭环做扎实：

- 看清：快速发现仓库，展示分支、远端、本地改动、领先/落后、冲突和远端关联风险。
- 看代码：从选中仓库直接打开 VS Code 或 IDEA，把代码审查、提交和冲突处理交还给编辑器。
- 同步安全：`fetch` 可自动或批量执行，`pull --ff-only` 默认只作用于干净仓库。
- 调整分支：查看本地/远端分支，并对当前仓库执行显式分支切换。
- 可验证：每轮改动都有对应的构建、测试、手动验收或发布检查。

在这些基础能力没有稳定前，仓库分组、后台托盘、深度性能架构、复杂发布体验等都应排在后面。

## 为什么做它

当一个工作区里有十几个甚至几十个 Git 仓库时，传统命令行工作流会变得碎片化：

- 每个仓库都要单独 `status`、`fetch`、`pull`
- 本地改动、远端落后状态分散在不同终端
- 想打开某个子仓库处理代码，需要反复切目录
- 批量 pull 容易误伤有本地改动的仓库

Fusion Git Desk 把这些动作收束成一个“多仓库驾驶舱”，重点是可见、可控、安全。

## 核心能力

- 自动扫描指定工作区，发现多层目录中的 Git 仓库
- 汇总展示每个仓库的当前分支、远端、ahead/behind、最后提交、本地改动数量
- 支持用 VS Code 或 IDEA 打开选中仓库
- 展示本地与远端分支，支持一键切换本地分支
- 支持单仓库或批量 `fetch`
- 支持单仓库或批量 `pull --ff-only`
- 支持定时刷新、自动 fetch、仅对干净仓库自动 pull
- Windows 下隐藏 Git 子进程窗口，避免刷新时弹出大量 Terminal
- 扫描与批量更新使用受控并发，适合多仓库工作区

## 安全策略

Fusion Git Desk 默认把“不会改工作区”的动作和“会改工作区”的动作区分开：

- `fetch` 只更新远端引用，适合自动执行
- `pull` 固定使用 `git pull --ff-only`，避免隐式 merge
- 默认仅对干净仓库执行 pull，保护未提交的本地改动
- 分支切换失败时直接展示 Git 原始错误，不会自动 stash 或覆盖文件

## 快速开始

本地开发需要：

- Go 1.23+
- Git
- Node.js 24+
- pnpm
- Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
cd frontend
pnpm install
cd ..
wails dev
```

## Windows 构建

```bash
wails build
```

产物路径：

```text
build/bin/FusionGitDesk.exe
```

## 本地自动部署

本地部署指在当前 Windows 机器生成可运行的桌面应用产物。脚本会依次运行前端构建、Go 测试和 Wails Windows 打包：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/deploy-local.ps1
```

启用本仓库的本地自动部署 hook 后，`main` 上的本地提交和 pull/merge 完成后会自动执行同一部署脚本：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/install-local-auto-deploy-hook.ps1
```

如需临时跳过某次本地自动部署，可在执行 Git 操作前设置 `FUSION_GIT_DESK_SKIP_LOCAL_DEPLOY=1`。
如果修改了 Wails 绑定入口并需要刷新前端绑定文件，可手动运行 `scripts/deploy-local.ps1 -RegenerateBindings`。

## macOS 打包

Wails v2 不能在 Windows 上交叉编译 macOS 应用。macOS 包需要在 Mac 或 GitHub Actions 的 macOS runner 上生成。

本仓库已经内置 macOS 打包脚本：

```bash
bash ./scripts/build-macos.sh
```

脚本会生成：

```text
build/dist/FusionGitDesk-macos-universal.app.zip
build/dist/FusionGitDesk-macos-universal.dmg
```

推送 `origin/main` 会自动触发 `Fusion Git Desk macOS Package` workflow 并上传 `.zip` 和 `.dmg` artifact；推送 `v*` 标签时还会创建 GitHub Release。普通远程分支不会触发自动打包。也可以在 GitHub Actions 中手动触发该 workflow。

## 项目结构

```text
.
├── app.go                    # Wails 绑定入口
├── git_service.go            # Git 扫描、状态、fetch/pull、分支和编辑器打开逻辑
├── settings.go               # 用户配置持久化
├── process_windows.go        # Windows 隐藏子进程窗口
├── frontend/                 # Vue 3 + Vite UI
├── scripts/deploy-local.ps1  # 本地 Windows 部署脚本
├── scripts/build-macos.sh    # macOS app/dmg 打包脚本
└── .github/workflows/        # macOS CI 打包 workflow
```

## 当前定位

Fusion Git Desk 不是一个完整替代 IDE Git 面板的工具，它更像一个轻量的多仓库运维台。它优先解决“很多仓库同时在手上时，如何快速知道全局状态、定位风险，并安全同步”的问题。

后续适合继续增强：

- Release 自动产物上传
- 签名与公证后的 macOS 分发包
- 仓库分组、标签、收藏
- 明确保护后的单仓库或批量推送
- 仓库收藏、分组和标签
