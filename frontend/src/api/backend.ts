import {
  CheckoutBranch,
  GetBranches,
  GetInitialState,
  GetRepositoryFileDiff,
  GetRepositoryDiff,
  PickDirectory,
  RefreshRepository,
  SaveSettings,
  ScanRepositories,
  UpdateRepositories,
} from '../../wailsjs/go/main/App'
import type {
  BranchResponse,
  CommandResult,
  DiffMode,
  DiffResponse,
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
  diffDisplayByteLimit: 900000,
}

const demoRepo: Repository = {
  id: 'demo',
  name: 'fusion-git-desk',
  path: 'E:\\my\\冷易达代码仓\\desktop\\fusion-git-desk',
  branch: 'main',
  head: 'local',
  upstream: 'origin/main',
  remoteUrl: 'git@example.com:fusion/git-desk.git',
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

function demoDiff(mode: DiffMode): DiffResponse {
  return {
    path: demoRepo.path,
    mode,
    raw: '',
    truncated: false,
    generated: new Date().toISOString(),
    files: [
      {
        oldPath: 'frontend/src/App.vue',
        newPath: 'frontend/src/App.vue',
        status: 'modified',
        additions: 3,
        deletions: 1,
        lines: [
          { kind: 'hunk', content: '@@ -12,7 +12,9 @@', oldLine: 0, newLine: 0 },
          { kind: 'context', content: 'const repositories = ref<Repository[]>([])', oldLine: 12, newLine: 12 },
          { kind: 'delete', content: 'const selected = ref("")', oldLine: 13, newLine: 0 },
          { kind: 'add', content: 'const selectedPath = ref("")', oldLine: 0, newLine: 13 },
          { kind: 'add', content: 'const activeDiffMode = ref<DiffMode>("working")', oldLine: 0, newLine: 14 },
        ],
      },
    ],
  }
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

  async scanRepositories(root: string, maxDepth: number): Promise<ScanResponse> {
    if (!hasBridge()) {
      return {
        root,
        maxDepth,
        repositories: [demoRepo],
        scannedAt: new Date().toISOString(),
      }
    }
    return ScanRepositories(root, maxDepth) as Promise<ScanResponse>
  },

  async refreshRepository(path: string): Promise<Repository> {
    if (!hasBridge()) return demoRepo
    return RefreshRepository(path) as Promise<Repository>
  },

  async getRepositoryDiff(path: string, mode: DiffMode): Promise<DiffResponse> {
    if (!hasBridge()) return demoDiff(mode)
    return GetRepositoryDiff(path, mode) as Promise<DiffResponse>
  },

  async getRepositoryFileDiff(path: string, mode: DiffMode, filePath: string): Promise<DiffResponse> {
    if (!hasBridge()) return { ...demoDiff(mode), target: filePath }
    return GetRepositoryFileDiff(path, mode, filePath) as Promise<DiffResponse>
  },

  async getBranches(path: string): Promise<BranchResponse> {
    if (!hasBridge()) {
      return {
        path,
        current: demoRepo.branch,
        generated: new Date().toISOString(),
        branches: [
          { name: 'main', current: true, remote: false, upstream: 'origin/main', commit: 'local', relativeTime: 'just now', subject: 'Create repository dashboard' },
          { name: 'feature/git-diff', current: false, remote: false, upstream: '', commit: 'a1b2c3d4', relativeTime: '2 hours ago', subject: 'Improve diff view' },
          { name: 'origin/main', current: false, remote: true, upstream: '', commit: 'e5f6a7b8', relativeTime: '1 day ago', subject: 'Remote baseline' },
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

  async updateRepositories(request: UpdateRequest): Promise<UpdateResult[]> {
    if (!hasBridge()) {
      return request.paths.map((path) => ({
        path,
        mode: request.mode,
        skipped: false,
        success: true,
        message: request.mode === 'pull' ? 'Already up to date.' : 'Fetched demo remotes.',
        stdout: '',
        stderr: '',
        finishedAt: new Date().toISOString(),
      }))
    }
    return UpdateRepositories(request) as Promise<UpdateResult[]>
  },
}
