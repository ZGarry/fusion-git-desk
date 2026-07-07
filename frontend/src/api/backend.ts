import {
  CheckoutBranch,
  CheckoutRemoteBranch,
  GetBranches,
  GetInitialState,
  OpenRepository,
  PickIdeaExecutable,
  PickDirectory,
  RefreshRepository,
  SaveSettings,
  ScanRepositories,
  UpdateRepositories,
} from '../../wailsjs/go/main/App'
import type {
  BranchResponse,
  CommandResult,
  InitialState,
  Repository,
  ScanResponse,
  Settings,
  UpdateRequest,
  UpdateResult,
} from '../types'

const defaultSettings: Settings = {
  lastRoot: '',
  maxDepth: 5,
  autoRefresh: false,
  refreshIntervalSeconds: 60,
  autoFetch: false,
  autoPullCleanRepos: false,
  onlyPullCleanRepos: true,
  ideaPath: '',
  diffDisplayByteLimit: 900000,
}

const demoRepo: Repository = {
  id: 'demo',
  name: 'fusion-git-desk',
  path: 'E:\\my\\冷易达代码仓\\desktop\\fusion-git-desk',
  branch: 'main',
  head: 'local',
  upstream: 'origin/main',
  remoteName: 'origin',
  remoteUrl: 'git@example.com:fusion/git-desk.git',
  hasRemote: true,
  hasUpstream: true,
  isClean: false,
  ahead: 1,
  behind: 2,
  inspectedAt: new Date().toISOString(),
  lastCommit: {
    hash: 'local',
    author: 'Codex',
    relativeTime: 'just now',
    subject: 'Create repository dashboard',
  },
  timings: {
    revParseMs: 4,
    statusMs: 18,
    remoteMs: 6,
    lastCommitMs: 9,
    totalMs: 42,
  },
  status: {
    added: 1,
    modified: 2,
    deleted: 0,
    renamed: 0,
    copied: 0,
    untracked: 1,
    conflicted: 0,
    staged: 1,
    unstaged: 2,
    files: [
      { path: 'app.go', status: 'M', staged: false, unstaged: true },
      { path: 'frontend/src/App.vue', status: 'M', staged: true, unstaged: false },
      { path: 'docs/requirements.md', status: '??', staged: false, unstaged: false },
    ],
  },
}

function hasBridge() {
  return Boolean((window as Window & { go?: unknown }).go)
}

export const api = {
  async getInitialState(): Promise<InitialState> {
    if (!hasBridge()) {
      return { settings: defaultSettings, hasGit: true }
    }
    return GetInitialState() as Promise<InitialState>
  },

  async saveSettings(settings: Settings): Promise<void> {
    if (!hasBridge()) return
    return SaveSettings(settings)
  },

  async pickDirectory(): Promise<string> {
    if (!hasBridge()) {
      return window.prompt('Workspace root') ?? ''
    }
    return PickDirectory()
  },

  async pickIdeaExecutable(): Promise<string> {
    if (!hasBridge()) {
      return window.prompt('IDEA executable') ?? ''
    }
    return PickIdeaExecutable()
  },

  async scanRepositories(root: string, maxDepth: number): Promise<ScanResponse> {
    if (!hasBridge()) {
      return {
        root,
        maxDepth,
        repositories: [demoRepo],
        scannedAt: new Date().toISOString(),
        warnings: [],
      }
    }
    return ScanRepositories(root, maxDepth) as Promise<ScanResponse>
  },

  async refreshRepository(path: string): Promise<Repository> {
    if (!hasBridge()) return demoRepo
    return RefreshRepository(path) as Promise<Repository>
  },

  async getBranches(path: string): Promise<BranchResponse> {
    if (!hasBridge()) {
      return {
        path,
        current: demoRepo.branch,
        generated: new Date().toISOString(),
        branches: [
          { name: 'main', current: true, remote: false, default: false, upstream: 'origin/main', commit: 'local', relativeTime: 'just now', subject: 'Create repository dashboard' },
          { name: 'feature/git-diff', current: false, remote: false, default: false, upstream: '', commit: 'a1b2c3d4', relativeTime: '2 hours ago', subject: 'Improve diff view' },
          { name: 'origin/main', current: false, remote: true, default: true, upstream: '', commit: 'e5f6a7b8', relativeTime: '1 day ago', subject: 'Remote baseline' },
        ],
      }
    }
    return GetBranches(path) as Promise<BranchResponse>
  },

  async checkoutBranch(path: string, branch: string): Promise<CommandResult> {
    if (!hasBridge()) {
      return { path, command: `git checkout ${branch}`, success: true, message: 'branch checked out', stdout: '', stderr: '', finishedAt: new Date().toISOString() }
    }
    return CheckoutBranch(path, branch) as Promise<CommandResult>
  },

  async checkoutRemoteBranch(path: string, branch: string): Promise<CommandResult> {
    if (!hasBridge()) {
      return { path, command: `git fetch && git checkout ${branch}`, success: true, message: `已拉取并切换到 ${branch.replace(/^[^/]+\//, '')}`, stdout: '', stderr: '', finishedAt: new Date().toISOString() }
    }
    return CheckoutRemoteBranch(path, branch) as Promise<CommandResult>
  },

  async openRepository(path: string, editor: 'vscode' | 'idea'): Promise<CommandResult> {
    if (!hasBridge()) {
      const label = editor === 'vscode' ? 'VS Code' : 'IDEA'
      return { path, command: label, success: true, message: `已用 ${label} 打开 ${path}`, stdout: '', stderr: '', finishedAt: new Date().toISOString() }
    }
    return OpenRepository(path, editor) as Promise<CommandResult>
  },

  async updateRepositories(request: UpdateRequest): Promise<UpdateResult[]> {
    if (!hasBridge()) {
      return request.paths.map((path) => ({
        path,
        mode: request.mode,
        skipped: request.mode === 'pull' && request.onlyClean && !demoRepo.isClean,
        success: !(request.mode === 'pull' && request.onlyClean && !demoRepo.isClean),
        message: request.mode === 'pull' && request.onlyClean && !demoRepo.isClean
          ? '工作区有本地改动，已按保护策略跳过拉取'
          : request.mode === 'pull' ? '已经是最新状态。' : '已检查远端状态。',
        stdout: '',
        stderr: '',
        before: demoRepo,
        after: demoRepo,
        finishedAt: new Date().toISOString(),
      }))
    }
    return UpdateRepositories(request) as Promise<UpdateResult[]>
  },
}
