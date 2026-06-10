# Fusion Git Desk 需求规划

## 产品定位

面向同时维护多个代码仓库的开发者、集成负责人和交付人员，提供一个桌面端多仓库 Git 工作台。它不替代专业 IDE，而是聚焦跨仓库状态聚合、批量更新、diff 快速巡检和分支观察。

## 核心能力

- 自动发现工作区下多个 Git 仓库
- 展示仓库状态、分支、远端、ahead/behind、最后提交
- 支持批量 fetch 和仅干净仓库 pull
- 支持 working/staged/HEAD diff
- 支持点击变更文件查看文件级 diff
- 支持对当前选中文件执行 stage/unstage，并在操作后刷新仓库状态和 diff
- 对未跟踪文本文件生成新增文件 diff 预览
- 对无文本 diff 的变更展示原因说明

## 后续路线

- 仓库收藏、分组、标签
- commit 草稿和 push
- 系统托盘和后台计划任务
- manifest 导入导出
