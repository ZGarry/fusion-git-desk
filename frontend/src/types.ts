export interface Settings {
  lastRoot: string
  maxDepth: number
  autoRefresh: boolean
  refreshIntervalSeconds: number
  autoFetch: boolean
  autoPullCleanRepos: boolean
  onlyPullCleanRepos: boolean
  diffDisplayByteLimit: number
}

export interface InitialState {
  settings: Settings
  hasGit: boolean
}

export interface ScanResponse {
  root: string
  maxDepth: number
  repositories: Repository[]
  scannedAt: string
  error?: string
}

export interface Repository {
  id: string
  name: string
  path: string
  branch: string
  head: string
  upstream: string
  remoteUrl: string
  hasUpstream: boolean
  isClean: boolean
  ahead: number
  behind: number
  status: RepoStatus
  lastCommit: CommitInfo
  inspectedAt: string
  error?: string
}

export interface RepoStatus {
  added: number
  modified: number
  deleted: number
  renamed: number
  copied: number
  untracked: number
  conflicted: number
  staged: number
  unstaged: number
  files: ChangedFile[]
}

export interface ChangedFile {
  path: string
  oldPath?: string
  status: string
  staged: boolean
  unstaged: boolean
}

export interface CommitInfo {
  hash: string
  author: string
  relativeTime: string
  subject: string
}

export interface BranchResponse {
  path: string
  current: string
  branches: BranchInfo[]
  generated: string
}

export interface BranchInfo {
  name: string
  current: boolean
  remote: boolean
  upstream: string
  commit: string
  relativeTime: string
  subject: string
}

export interface DiffResponse {
  path: string
  mode: DiffMode
  target?: string
  files: DiffFile[]
  raw: string
  truncated: boolean
  generated: string
  note?: string
  error?: string
}

export interface DiffFile {
  oldPath: string
  newPath: string
  status: string
  additions: number
  deletions: number
  lines: DiffLine[]
}

export interface DiffLine {
  kind: 'add' | 'delete' | 'context' | 'hunk' | 'meta'
  content: string
  oldLine: number
  newLine: number
}

export type DiffMode = 'working' | 'staged' | 'head'
export type UpdateMode = 'fetch' | 'pull'

export interface UpdateRequest {
  paths: string[]
  mode: UpdateMode
  onlyClean: boolean
  prune: boolean
}

export interface UpdateResult {
  path: string
  mode: UpdateMode
  skipped: boolean
  success: boolean
  message: string
  stdout: string
  stderr: string
  before?: Repository
  after?: Repository
  finishedAt: string
}

export interface CommandResult {
  path: string
  command: string
  success: boolean
  message: string
  stdout: string
  stderr: string
  finishedAt: string
}
