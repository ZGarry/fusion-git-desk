<script setup lang="ts">
import {
  AlertTriangle,
  Check,
  ChevronRight,
  Download,
  FolderOpen,
  GitBranch,
  GitCompare,
  GitPullRequest,
  Minus,
  Pause,
  Play,
  Plus,
  RefreshCw,
  RotateCw,
  Search,
} from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { api } from './api/backend'
import type {
  BranchInfo,
  BranchResponse,
  ChangedFile,
  DiffFile,
  DiffMode,
  DiffResponse,
  Repository,
  Settings,
  UpdateMode,
  UpdateResult,
} from './types'

type AdvisoryTone = 'danger' | 'warn' | 'info' | 'ok'

interface RepoAdvisory {
  tone: AdvisoryTone
  label: string
  detail: string
}

const settings = reactive<Settings>({
  lastRoot: '',
  maxDepth: 5,
  autoRefresh: false,
  refreshIntervalSeconds: 60,
  autoFetch: false,
  autoPullCleanRepos: false,
  onlyPullCleanRepos: true,
  diffDisplayByteLimit: 900000,
})

const state = reactive({
  hasGit: true,
  booting: true,
  scanning: false,
  updating: false,
  loadingDiff: false,
  loadingBranches: false,
  checkingOut: '',
  indexingFile: '',
  error: '',
  notice: '',
  lastScan: '',
  autoCycleNote: '自动刷新已暂停',
  autoCycleFailureCount: 0,
  autoCycleLastError: '',
  autoCycleFailureTimes: [] as number[],
})

const rootInput = ref('')
const repoFilter = ref('')
const changedFileFilter = ref('')
const repositories = ref<Repository[]>([])
const selectedPath = ref('')
const selectedFilePath = ref('')
const activeDiffMode = ref<DiffMode>('working')
const diff = ref<DiffResponse | null>(null)
const branches = ref<BranchResponse | null>(null)
const updateResults = ref<UpdateResult[]>([])

const maxRenderedChangedFiles = 600
const maxRenderedDiffFiles = 80
const maxRenderedDiffLines = 4000
const autoCycleFailureWindowMs = 10 * 60 * 1000

let refreshTimer: number | undefined
let settingsSaveTimer: number | undefined
let diffRequestId = 0
let branchesRequestId = 0

const filteredRepositories = computed(() => {
  const keyword = repoFilter.value.trim().toLowerCase()
  const list = keyword
    ? repositories.value.filter((repo) => `${repo.name} ${repo.path} ${repo.branch}`.toLowerCase().includes(keyword))
    : repositories.value

  return [...list].sort((a, b) => {
    const priority = repoAttentionRank(b) - repoAttentionRank(a)
    if (priority !== 0) return priority
    if (a.isClean !== b.isClean) return a.isClean ? 1 : -1
    if (a.behind !== b.behind) return b.behind - a.behind
    return a.name.localeCompare(b.name)
  })
})

const repositoryByPath = computed(() => {
  const byPath = new Map<string, Repository>()
  for (const repo of repositories.value) {
    byPath.set(repo.path, repo)
  }
  return byPath
})

const selectedRepo = computed(() => repositoryByPath.value.get(selectedPath.value) ?? null)

const repoStats = computed(() => {
  const stats = {
    total: repositories.value.length,
    dirty: 0,
    conflicted: 0,
    behind: 0,
    ahead: 0,
    noUpstream: 0,
    pullBlocked: 0,
  }
  for (const repo of repositories.value) {
    if (!repo.isClean) stats.dirty++
    if (repo.status.conflicted > 0) stats.conflicted++
    if (repo.behind > 0) stats.behind++
    if (repo.ahead > 0) stats.ahead++
    if (!repo.hasUpstream) stats.noUpstream++
    if (isPullProtected(repo)) stats.pullBlocked++
  }
  return stats
})

const selectedFiles = computed(() => selectedRepo.value?.status.files ?? [])
const selectedFile = computed(() => selectedFiles.value.find((file) => file.path === selectedFilePath.value) ?? null)
const selectedPullWarning = computed(() => {
  if (!selectedRepo.value) return ''
  if (selectedRepo.value.status.conflicted > 0) {
    return '当前仓库存在冲突，建议先解决冲突后再 Pull 或切换分支。'
  }
  if (!selectedRepo.value.hasUpstream) {
    return '当前分支没有 upstream，Pull 会跳过；Fetch 仍可查看远端分支。'
  }
  if (selectedRepo.value.isClean || !settings.onlyPullCleanRepos) return ''
  return '当前仓库有本地改动，Pull 会跳过；Fetch 仍可更新远端引用。'
})
const selectedRepoAdvisories = computed(() => selectedRepo.value ? buildRepoAdvisories(selectedRepo.value) : [])
const selectedRepoHasDanger = computed(() => selectedRepoAdvisories.value.some((item) => item.tone === 'danger'))
const gitActionBusy = computed(() => state.scanning || state.updating || Boolean(state.indexingFile))
const selectedFileCanStage = computed(() => Boolean(selectedFile.value && (selectedFile.value.unstaged || selectedFile.value.status === '??')))
const selectedFileCanUnstage = computed(() => Boolean(selectedFile.value?.staged))
const fetchAllTitle = computed(() => '全部 Fetch：更新远端引用，不合并到本地工作区。')
const pullAllTitle = computed(() => summarizePullPreflight(repositories.value, '全部 Pull'))
const selectedPullTitle = computed(() => selectedRepo.value ? summarizePullPreflight([selectedRepo.value], 'Pull') : 'Pull selected')
const changedFileKeyword = computed(() => changedFileFilter.value.trim().toLowerCase())
const filteredSelectedFiles = computed(() => {
  const keyword = changedFileKeyword.value
  if (!keyword) return selectedFiles.value
  return selectedFiles.value.filter((file) => `${file.status} ${file.path} ${file.oldPath ?? ''}`.toLowerCase().includes(keyword))
})
const renderedSelectedFiles = computed(() => filteredSelectedFiles.value.slice(0, maxRenderedChangedFiles))
const hiddenSelectedFileCount = computed(() => Math.max(0, filteredSelectedFiles.value.length - renderedSelectedFiles.value.length))
const hiddenByFileFilterCount = computed(() => Math.max(0, selectedFiles.value.length - filteredSelectedFiles.value.length))
const localBranches = computed(() => (branches.value?.branches ?? []).filter((branch) => !branch.remote))
const remoteBranches = computed(() => (branches.value?.branches ?? []).filter((branch) => branch.remote))
const renderedDiff = computed(() => buildRenderedDiff(diff.value))
const autoCycleAlert = computed(() => {
  if (state.autoCycleFailureCount === 0) return ''
  const detail = state.autoCycleLastError ? `：${state.autoCycleLastError}` : ''
  return `连续失败 ${state.autoCycleFailureCount} 次${detail}`
})
const autoCycleWindowAlert = computed(() => {
  const recentFailureCount = recentAutoCycleFailureCount()
  if (recentFailureCount <= state.autoCycleFailureCount) return ''
  return `近 10 分钟失败 ${recentFailureCount} 次（含已恢复）`
})
const autoCycleSeverityClass = computed(() => {
  if (state.autoCycleFailureCount >= 3) return 'danger'
  if (state.autoCycleFailureCount > 0 || autoCycleWindowAlert.value || state.autoCycleNote.includes('跳过')) return 'warn'
  return ''
})

onMounted(async () => {
  try {
    const initial = await api.getInitialState()
    Object.assign(settings, normalizeSettings(initial.settings))
    state.hasGit = initial.hasGit
    rootInput.value = settings.lastRoot
    if (settings.lastRoot) {
      await scanRepositories(false)
    }
  } catch (error) {
    state.error = messageOf(error)
  } finally {
    state.booting = false
    setupRefreshTimer()
  }
})

onBeforeUnmount(() => {
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
  if (settingsSaveTimer) {
    window.clearTimeout(settingsSaveTimer)
    void api.saveSettings({ ...settings })
  }
})

watch(selectedPath, () => {
  selectedFilePath.value = ''
  changedFileFilter.value = ''
  void loadSelectedDetails()
})

watch(activeDiffMode, () => {
  void loadDiff()
})

watch(
  () => [settings.autoRefresh, settings.refreshIntervalSeconds, settings.autoFetch, settings.autoPullCleanRepos] as const,
  () => {
    if (!state.booting) {
      setupRefreshTimer()
    }
  },
)

watch(
  settings,
  () => {
    if (!state.booting) {
      queueSaveSettings()
    }
  },
  { deep: true },
)

async function chooseRoot() {
  const picked = await api.pickDirectory()
  if (picked) {
    rootInput.value = picked
    await scanRepositories(true)
  }
}

async function scanRepositories(forceNotice = true) {
  const root = rootInput.value.trim()
  if (!root) {
    state.error = '请输入或选择工作区目录'
    return false
  }
  if (guardBusyAction('扫描', forceNotice)) {
    return false
  }

  state.error = ''
  state.scanning = true
  try {
    const response = await api.scanRepositories(root, settings.maxDepth)
    repositories.value = response.repositories ?? []
    state.lastScan = response.scannedAt
    settings.lastRoot = response.root
    settings.maxDepth = response.maxDepth
    if (response.error) {
      state.error = response.error
    }

    if (!selectedPath.value || !repositories.value.some((repo) => repo.path === selectedPath.value)) {
      selectedPath.value = repositories.value[0]?.path ?? ''
    } else {
      void loadSelectedDetails()
    }

    if (forceNotice) {
      state.notice = `发现 ${repositories.value.length} 个仓库`
    }
    return !response.error
  } catch (error) {
    state.error = messageOf(error)
    return false
  } finally {
    state.scanning = false
  }
}

async function refreshSelected() {
  if (!selectedRepo.value) return false
  if (guardBusyAction('仓库刷新')) {
    return false
  }
  state.error = ''
  state.scanning = true
  try {
    const repo = await api.refreshRepository(selectedRepo.value.path)
    replaceRepository(repo)
    selectedPath.value = repo.path
    void loadSelectedDetails()
    state.notice = `${repo.name} 已刷新`
    return true
  } catch (error) {
    state.error = messageOf(error)
    return false
  } finally {
    state.scanning = false
  }
}

async function loadSelectedDetails() {
  if (!selectedRepo.value) {
    diff.value = null
    branches.value = null
    return
  }

  await Promise.all([loadDiff(), loadBranches()])
}

async function loadDiff() {
  if (!selectedRepo.value) {
    diffRequestId++
    diff.value = null
    state.loadingDiff = false
    return
  }

  const repoPath = selectedRepo.value.path
  const mode = activeDiffMode.value
  const filePath = selectedFilePath.value
  const requestId = ++diffRequestId
  state.loadingDiff = true
  try {
    const nextDiff = filePath
      ? await api.getRepositoryFileDiff(repoPath, mode, filePath)
      : await api.getRepositoryDiff(repoPath, mode)
    if (isCurrentDiffRequest(requestId, repoPath, mode, filePath)) {
      diff.value = nextDiff
    }
  } catch (error) {
    if (requestId === diffRequestId) {
      state.error = messageOf(error)
    }
  } finally {
    if (requestId === diffRequestId) {
      state.loadingDiff = false
    }
  }
}

async function loadBranches() {
  if (!selectedRepo.value) {
    branchesRequestId++
    branches.value = null
    state.loadingBranches = false
    return
  }

  const repoPath = selectedRepo.value.path
  const requestId = ++branchesRequestId
  state.loadingBranches = true
  try {
    const nextBranches = await api.getBranches(repoPath)
    if (requestId === branchesRequestId && selectedRepo.value?.path === repoPath) {
      branches.value = nextBranches
    }
  } catch (error) {
    if (requestId === branchesRequestId) {
      state.error = messageOf(error)
    }
  } finally {
    if (requestId === branchesRequestId) {
      state.loadingBranches = false
    }
  }
}

async function updateRepositories(mode: UpdateMode, scope: 'selected' | 'all' = 'selected', silent = false) {
  const paths = scope === 'all'
    ? repositories.value.map((repo) => repo.path)
    : selectedRepo.value
      ? [selectedRepo.value.path]
      : []

  if (paths.length === 0) return null
  if (state.scanning) {
    if (!silent) {
      state.notice = `扫描或刷新进行中，已跳过${mode === 'pull' ? '拉取' : '获取'}请求`
    }
    return null
  }
  if (state.updating) {
    if (!silent) {
      state.notice = `已有批量更新进行中，已跳过${mode === 'pull' ? '拉取' : '获取'}请求`
    }
    return null
  }
  if (state.indexingFile) {
    if (!silent) {
      state.notice = `文件暂存操作进行中，已跳过${mode === 'pull' ? '拉取' : '获取'}请求`
    }
    return null
  }

  state.error = ''
  state.updating = true
  try {
    const results = await api.updateRepositories({
      paths,
      mode,
      onlyClean: mode === 'pull' ? settings.onlyPullCleanRepos : false,
      prune: true,
    })
    updateResults.value = results
    for (const result of results) {
      if (result.after) {
        replaceRepository(result.after)
      }
    }
    void loadSelectedDetails()
    if (!silent) {
      state.notice = summarizeUpdate(results, mode)
    }
    return results
  } catch (error) {
    state.error = messageOf(error)
    return null
  } finally {
    state.updating = false
  }
}

async function checkoutBranch(branch: BranchInfo) {
  if (!selectedRepo.value || branch.remote || branch.current) return
  if (guardBusyAction('切换分支')) {
    return
  }

  state.checkingOut = branch.name
  state.error = ''
  try {
    const result = await api.checkoutBranch(selectedRepo.value.path, branch.name)
    if (!result.success) {
      state.error = result.message
      return
    }
    state.notice = result.message
    await refreshSelected()
  } catch (error) {
    state.error = messageOf(error)
  } finally {
    state.checkingOut = ''
  }
}

async function stageSelectedFile() {
  await mutateSelectedFileIndex('stage')
}

async function unstageSelectedFile() {
  await mutateSelectedFileIndex('unstage')
}

async function mutateSelectedFileIndex(action: 'stage' | 'unstage') {
  if (!selectedRepo.value || !selectedFile.value) return
  if (guardBusyAction(action === 'stage' ? '暂存文件' : '取消暂存')) {
    return
  }

  const repoPath = selectedRepo.value.path
  const filePath = selectedFile.value.path
  state.error = ''
  state.indexingFile = filePath
  try {
    const result = action === 'stage'
      ? await api.stageFile(repoPath, filePath)
      : await api.unstageFile(repoPath, filePath)
    if (!result.success) {
      state.error = result.message
      return
    }

    const refreshed = await api.refreshRepository(result.path || repoPath)
    replaceRepository(refreshed)
    selectedPath.value = refreshed.path
    const fileStillChanged = refreshed.status.files.some((file) => file.path === filePath || file.oldPath === filePath)
    if (!fileStillChanged) {
      selectedFilePath.value = ''
    }
    const previousMode = activeDiffMode.value
    activeDiffMode.value = action === 'stage' ? 'staged' : 'working'
    if (previousMode === activeDiffMode.value) {
      void loadDiff()
    }
    state.notice = result.message
  } catch (error) {
    state.error = messageOf(error)
  } finally {
    state.indexingFile = ''
  }
}

function setupRefreshTimer() {
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
    refreshTimer = undefined
  }

  if (!settings.autoRefresh) {
    state.autoCycleNote = '自动刷新已暂停'
    clearAutoCycleFailure(true)
    return
  }
  state.autoCycleNote = `${autoCycleLabel()}每 ${settings.refreshIntervalSeconds} 秒运行`
  refreshTimer = window.setInterval(() => {
    void runAutoCycle()
  }, settings.refreshIntervalSeconds * 1000)
}

function queueSaveSettings() {
  if (settingsSaveTimer) {
    window.clearTimeout(settingsSaveTimer)
  }
  settingsSaveTimer = window.setTimeout(() => {
    settingsSaveTimer = undefined
    void api.saveSettings({ ...settings })
  }, 250)
}

function isCurrentDiffRequest(requestId: number, repoPath: string, mode: DiffMode, filePath: string) {
  return requestId === diffRequestId
    && selectedRepo.value?.path === repoPath
    && activeDiffMode.value === mode
    && selectedFilePath.value === filePath
}

async function runAutoCycle() {
  const modeLabel = autoCycleLabel()
  if (!rootInput.value.trim()) {
    state.autoCycleNote = `${modeLabel}待命：未设置工作区`
    clearAutoCycleFailure()
    return
  }
  if (repositories.value.length === 0) {
    state.autoCycleNote = `${modeLabel}待命：当前没有仓库`
    clearAutoCycleFailure()
    return
  }
  if (state.scanning) {
    state.autoCycleNote = `${modeLabel}跳过：上一次扫描或刷新未完成`
    return
  }
  if (state.updating) {
    state.autoCycleNote = `${modeLabel}跳过：批量更新进行中`
    return
  }

  const finishedAt = formatDate(new Date().toISOString())
  if (settings.autoPullCleanRepos) {
    const results = await updateRepositories('pull', 'all', true)
    if (!results) {
      recordAutoCycleFailure(`${modeLabel}结束于 ${finishedAt}`, state.error || '自动拉取未返回结果')
      return
    }
    state.autoCycleNote = summarizeAutoCycleUpdate(modeLabel, results, 'pull', finishedAt)
    const failureMessage = summarizeAutoCycleFailure(results)
    if (failureMessage) {
      recordAutoCycleFailure(state.autoCycleNote, failureMessage)
      return
    }
    clearAutoCycleFailure()
  } else if (settings.autoFetch) {
    const results = await updateRepositories('fetch', 'all', true)
    if (!results) {
      recordAutoCycleFailure(`${modeLabel}结束于 ${finishedAt}`, state.error || '自动获取未返回结果')
      return
    }
    state.autoCycleNote = summarizeAutoCycleUpdate(modeLabel, results, 'fetch', finishedAt)
    const failureMessage = summarizeAutoCycleFailure(results)
    if (failureMessage) {
      recordAutoCycleFailure(state.autoCycleNote, failureMessage)
      return
    }
    clearAutoCycleFailure()
  } else {
    const ok = await scanRepositories(false)
    state.autoCycleNote = `${modeLabel}${ok ? '完成' : '结束'}于 ${finishedAt}`
    if (!ok) {
      recordAutoCycleFailure(state.autoCycleNote, state.error || '自动刷新扫描失败')
      return
    }
    clearAutoCycleFailure()
  }
}

function replaceRepository(repo: Repository) {
  const index = repositories.value.findIndex((item) => item.path === repo.path)
  if (index >= 0) {
    repositories.value.splice(index, 1, repo)
  } else {
    repositories.value.push(repo)
  }
}

function setDiffMode(mode: DiffMode) {
  activeDiffMode.value = mode
}

function selectChangedFile(file: ChangedFile) {
  selectedFilePath.value = file.path
  const previousMode = activeDiffMode.value
  if (file.status === '??' || file.unstaged) {
    activeDiffMode.value = 'working'
  } else if (file.staged) {
    activeDiffMode.value = 'staged'
  }
  if (previousMode === activeDiffMode.value) {
    void loadDiff()
  }
}

function clearSelectedFile() {
  selectedFilePath.value = ''
  void loadDiff()
}

function changedCount(repo: Repository) {
  return repo.status.files.length
}

function repoAttentionRank(repo: Repository) {
  let score = 0
  if (repo.error) score += 10000
  if (repo.status.conflicted > 0) score += 8000
  if (!repo.hasUpstream) score += 3000
  if (repo.behind > 0 && isPullProtected(repo)) score += 2600
  else if (repo.behind > 0) score += 2200
  if (!repo.isClean) score += 1200
  if (repo.ahead > 0) score += 600
  return score
}

function isPullProtected(repo: Repository) {
  return settings.onlyPullCleanRepos && repo.hasUpstream && repo.status.conflicted === 0 && !repo.isClean
}

function buildRepoAdvisories(repo: Repository): RepoAdvisory[] {
  const advisories: RepoAdvisory[] = []

  if (repo.error) {
    advisories.push({ tone: 'danger', label: '扫描异常', detail: repo.error })
  }
  if (repo.status.conflicted > 0) {
    advisories.push({
      tone: 'danger',
      label: `存在冲突 ${repo.status.conflicted}`,
      detail: '先解决冲突并刷新仓库，再执行 Pull、切换分支或提交。',
    })
  }
  if (!repo.hasUpstream) {
    advisories.push({
      tone: 'warn',
      label: '无 upstream',
      detail: '当前分支没有跟踪分支；Fetch 可更新远端引用，Pull 会跳过并提示先设置 upstream。',
    })
  }
  if (repo.behind > 0 && repo.hasUpstream) {
    if (isPullProtected(repo)) {
      advisories.push({
        tone: 'warn',
        label: `落后 ${repo.behind} 且有本地改动`,
        detail: '当前保护策略会跳过 Pull；可以先查看 diff、提交/暂存改动，或只执行 Fetch。',
      })
    } else if (repo.isClean) {
      advisories.push({
        tone: 'info',
        label: `可 Pull ${repo.behind}`,
        detail: '工作区干净，适合执行 fast-forward Pull。',
      })
    } else {
      advisories.push({
        tone: 'warn',
        label: `落后 ${repo.behind} 且工作区不干净`,
        detail: '已关闭“仅干净仓库 Pull”，执行 Pull 前建议先确认本地 diff。',
      })
    }
  }
  if (repo.ahead > 0) {
    advisories.push({
      tone: 'info',
      label: `本地领先 ${repo.ahead}`,
      detail: '本地有提交尚未进入 upstream；检查后可按单仓库粒度 Push。',
    })
  }
  if (!repo.isClean && repo.status.conflicted === 0) {
    advisories.push({
      tone: 'warn',
      label: '工作区有改动',
      detail: formatStatusBreakdown(repo.status),
    })
  }
  if (repo.isClean && repo.hasUpstream && repo.ahead === 0 && repo.behind === 0 && !repo.error) {
    advisories.push({
      tone: 'ok',
      label: '工作区干净',
      detail: '当前分支与 upstream 没有已知领先/落后，可继续查看代码或 Fetch 更新远端引用。',
    })
  }

  return advisories
}

function formatStatusBreakdown(status: Repository['status']) {
  const parts = [
    status.conflicted > 0 ? `冲突 ${status.conflicted}` : '',
    status.staged > 0 ? `已暂存 ${status.staged}` : '',
    status.unstaged > 0 ? `未暂存 ${status.unstaged}` : '',
    status.untracked > 0 ? `未跟踪 ${status.untracked}` : '',
  ].filter(Boolean)
  return parts.length ? parts.join('，') : `${status.files.length} 个变更文件`
}

function summarizePullPreflight(repos: Repository[], action: string) {
  if (!repos.length) return `${action}：当前没有仓库`

  const conflicted = repos.filter((repo) => repo.status.conflicted > 0).length
  const noUpstream = repos.filter((repo) => !repo.hasUpstream).length
  const protectedDirty = repos.filter((repo) => isPullProtected(repo)).length
  const cleanBehind = repos.filter((repo) => repo.hasUpstream && repo.isClean && repo.behind > 0 && repo.status.conflicted === 0).length
  const parts = [
    cleanBehind > 0 ? `${cleanBehind} 个落后且干净` : '',
    protectedDirty > 0 ? `${protectedDirty} 个有本地改动会跳过` : '',
    noUpstream > 0 ? `${noUpstream} 个无 upstream 会跳过` : '',
    conflicted > 0 ? `${conflicted} 个有冲突会跳过` : '',
  ].filter(Boolean)

  return `${action}：${parts.length ? parts.join('；') : '执行 git pull --ff-only'}`
}

function branchCheckoutTitle(branch: BranchInfo) {
  if (branch.current) return '当前分支'
  if (selectedRepo.value?.status.conflicted) return '存在冲突，建议先解决冲突后再切换分支'
  if (selectedRepo.value && !selectedRepo.value.isClean) return '有本地改动，切换分支可能被 Git 拒绝以保护文件'
  return `切换到 ${branch.name}`
}

function formatDurationMs(value?: number) {
  const milliseconds = Number(value ?? 0)
  if (!Number.isFinite(milliseconds) || milliseconds <= 0) return '0ms'
  if (milliseconds >= 1000) {
    const seconds = milliseconds / 1000
    return `${seconds >= 10 ? seconds.toFixed(0) : seconds.toFixed(1)}s`
  }
  return `${Math.round(milliseconds)}ms`
}

function scanTimingTitle(repo: Repository) {
  const timings = repo.timings
  if (!timings) return ''
  return [
    `rev-parse ${formatDurationMs(timings.revParseMs)}`,
    `status ${formatDurationMs(timings.statusMs)}`,
    `remote ${formatDurationMs(timings.remoteMs)}`,
    `log ${formatDurationMs(timings.lastCommitMs)}`,
  ].join(' | ')
}

function guardBusyAction(action: string, showNotice = true) {
  let reason = ''
  if (state.scanning) {
    reason = '已有扫描或刷新进行中'
  } else if (state.updating) {
    reason = '批量更新进行中'
  } else if (state.indexingFile) {
    reason = '文件暂存操作进行中'
  }
  if (!reason) {
    return false
  }
  if (showNotice) {
    state.notice = `${reason}，已跳过${action}`
  }
  return true
}

function autoCycleLabel() {
  if (settings.autoPullCleanRepos) return 'Auto Pull'
  if (settings.autoFetch) return 'Auto Fetch'
  return '自动刷新'
}

function clearAutoCycleFailure(resetHistory = false) {
  state.autoCycleFailureCount = 0
  state.autoCycleLastError = ''
  if (resetHistory) {
    state.autoCycleFailureTimes = []
  } else {
    pruneAutoCycleFailureTimes()
  }
}

function recordAutoCycleFailure(note: string, errorMessage: string) {
  const now = Date.now()
  pruneAutoCycleFailureTimes(now)
  state.autoCycleFailureTimes.push(now)
  state.autoCycleNote = note
  state.autoCycleFailureCount++
  state.autoCycleLastError = errorMessage
}

function recentAutoCycleFailureCount() {
  const cutoff = Date.now() - autoCycleFailureWindowMs
  return state.autoCycleFailureTimes.filter((failedAt) => failedAt >= cutoff).length
}

function pruneAutoCycleFailureTimes(now = Date.now()) {
  const cutoff = now - autoCycleFailureWindowMs
  state.autoCycleFailureTimes = state.autoCycleFailureTimes.filter((failedAt) => failedAt >= cutoff)
}

function buildRenderedDiff(source: DiffResponse | null) {
  if (!source) {
    return null
  }

  const files: DiffFile[] = []
  let renderedLines = 0
  let hiddenLines = 0
  let remainingLines = maxRenderedDiffLines

  for (const file of source.files) {
    if (files.length >= maxRenderedDiffFiles) {
      hiddenLines += file.lines.length
      continue
    }

    const lineCount = file.lines.length
    const lines = remainingLines > 0 ? file.lines.slice(0, remainingLines) : []
    renderedLines += lines.length
    hiddenLines += lineCount - lines.length
    remainingLines -= lines.length
    files.push(lines.length === lineCount ? file : { ...file, lines })
  }

  return {
    ...source,
    files,
    hiddenFiles: Math.max(0, source.files.length - files.length),
    hiddenLines,
    renderedLines,
  }
}

function formatDate(value: string) {
  if (!value) return ''
  return new Date(value).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function normalizeSettings(value: Settings): Settings {
  return {
    ...settings,
    ...value,
    maxDepth: clampNumber(value.maxDepth || 5, 1, 12),
    refreshIntervalSeconds: clampNumber(value.refreshIntervalSeconds || 60, 15, 3600),
    diffDisplayByteLimit: value.diffDisplayByteLimit || 900000,
  }
}

function clampNumber(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function summarizeUpdate(results: UpdateResult[], mode: UpdateMode) {
  const { success, skipped, failed, verb } = summarizeUpdateCounts(results, mode)
  const detail = summarizeUpdateDetail(results, mode)
  return `${verb}完成：成功 ${success}，跳过 ${skipped}，失败 ${failed}${detail ? `；${detail}` : ''}`
}

function summarizeAutoCycleUpdate(modeLabel: string, results: UpdateResult[], mode: UpdateMode, finishedAt: string) {
  const { success, skipped, failed, verb } = summarizeUpdateCounts(results, mode)
  const status = failed > 0 ? '结束' : '完成'
  const detail = summarizeUpdateDetail(results, mode)
  return `${modeLabel}${status}于 ${finishedAt}：${verb}成功 ${success}，跳过 ${skipped}，失败 ${failed}${detail ? `；${detail}` : ''}`
}

function summarizeAutoCycleFailure(results: UpdateResult[]) {
  const failedResult = results.find((result) => !result.success && !result.skipped)
  return failedResult?.message || ''
}

function summarizeUpdateCounts(results: UpdateResult[], mode: UpdateMode) {
  const success = results.filter((result) => result.success).length
  const skipped = results.filter((result) => result.skipped).length
  const failed = results.length - success - skipped
  const verb = mode === 'pull' ? '拉取' : '获取'
  return { success, skipped, failed, verb }
}

function summarizeUpdateDetail(results: UpdateResult[], mode: UpdateMode) {
  const skipped = results.filter((result) => result.skipped)
  const failed = results.filter((result) => !result.success && !result.skipped)
  if (skipped.length && mode === 'pull') {
    const dirtySkipped = skipped.filter((result) => isDirtyWorkingTreeSkip(result))
    if (dirtySkipped.length === skipped.length) {
      return skipped.length === 1
        ? '工作区有本地改动，已跳过 Pull；可先 Fetch 更新远端引用'
        : `${skipped.length} 个仓库有本地改动，已跳过 Pull；可先 Fetch 更新远端引用`
    }
    return firstUsefulMessage(skipped) || '部分仓库被跳过'
  }
  if (failed.length) {
    return firstUsefulMessage(failed)
  }
  return ''
}

function isDirtyWorkingTreeSkip(result: UpdateResult) {
  return result.message.toLowerCase().includes('working tree has local changes') || result.message.includes('本地改动')
}

function firstUsefulMessage(results: UpdateResult[]) {
  return results.find((result) => result.message.trim())?.message.trim() ?? ''
}

function messageOf(error: unknown) {
  if (error instanceof Error) return error.message
  return String(error)
}
</script>

<template>
  <main class="app-shell">
    <header class="topbar">
      <div class="brand">
        <GitCompare :size="26" />
        <div>
          <h1>Fusion Git Desk</h1>
          <p>多仓库工作台</p>
        </div>
      </div>

      <div class="root-picker">
        <button class="icon-button" title="选择目录" @click="chooseRoot">
          <FolderOpen :size="18" />
        </button>
        <input v-model="rootInput" spellcheck="false" placeholder="工作区目录" @keydown.enter="scanRepositories(true)" />
        <label class="depth-field">
          深度
          <input v-model.number="settings.maxDepth" min="1" max="12" type="number" />
        </label>
        <button class="primary-button" :disabled="gitActionBusy" @click="scanRepositories(true)">
          <RefreshCw :size="17" :class="{ spin: state.scanning }" />
          扫描
        </button>
      </div>

      <div class="top-actions">
        <button :title="fetchAllTitle" :disabled="gitActionBusy || repositories.length === 0" @click="updateRepositories('fetch', 'all')">
          <Download :size="17" />
          全部 Fetch
        </button>
        <button :title="pullAllTitle" :disabled="gitActionBusy || repositories.length === 0" @click="updateRepositories('pull', 'all')">
          <GitPullRequest :size="17" />
          全部 Pull
        </button>
      </div>
    </header>

    <section class="control-strip">
      <div class="metric">
        <span>{{ repoStats.total }}</span>
        仓库
      </div>
      <div class="metric warn">
        <span>{{ repoStats.dirty }}</span>
        有改动
      </div>
      <div class="metric danger">
        <span>{{ repoStats.conflicted }}</span>
        冲突
      </div>
      <div class="metric info">
        <span>{{ repoStats.behind }}</span>
        落后
      </div>
      <div class="metric">
        <span>{{ repoStats.ahead }}</span>
        领先
      </div>
      <div class="metric warn">
        <span>{{ repoStats.noUpstream }}</span>
        无上游
      </div>
      <div class="metric warn">
        <span>{{ repoStats.pullBlocked }}</span>
        Pull保护
      </div>

      <div class="toggles">
        <label>
          <input v-model="settings.autoRefresh" type="checkbox" />
          {{ settings.autoRefresh ? '自动刷新' : '手动刷新' }}
        </label>
        <label>
          <input v-model="settings.autoFetch" type="checkbox" />
          Auto Fetch
        </label>
        <label>
          <input v-model="settings.autoPullCleanRepos" type="checkbox" />
          Auto Pull
        </label>
        <label>
          间隔
          <input v-model.number="settings.refreshIntervalSeconds" class="small-number" min="15" max="3600" type="number" />
          秒
        </label>
        <label>
          <input v-model="settings.onlyPullCleanRepos" type="checkbox" />
          仅干净仓库 Pull
        </label>
      </div>

      <div class="scan-time">
        <Play v-if="settings.autoRefresh" :size="15" />
        <Pause v-else :size="15" />
        <div class="scan-time-copy">
          <span>{{ state.lastScan ? `上次扫描 ${formatDate(state.lastScan)}` : '未扫描' }}</span>
          <span class="scan-time-note" :class="autoCycleSeverityClass">{{ state.autoCycleNote }}</span>
          <span v-if="autoCycleAlert" class="scan-time-note" :class="autoCycleSeverityClass">{{ autoCycleAlert }}</span>
          <span v-if="autoCycleWindowAlert" class="scan-time-note warn">{{ autoCycleWindowAlert }}</span>
        </div>
      </div>
    </section>

    <section v-if="!state.hasGit" class="banner danger">
      <AlertTriangle :size="18" />
      未找到 Git，请先安装 Git 并加入 PATH。
    </section>
    <section v-else-if="state.error" class="banner danger">
      <AlertTriangle :size="18" />
      {{ state.error }}
    </section>
    <section v-else-if="state.notice" class="banner">
      <Check :size="18" />
      {{ state.notice }}
    </section>

    <div class="workspace">
      <aside class="repo-panel">
        <div class="panel-head">
          <h2>仓库</h2>
          <div class="filter">
            <Search :size="16" />
            <input v-model="repoFilter" placeholder="过滤" />
          </div>
        </div>

        <div class="repo-list">
          <button
            v-for="repo in filteredRepositories"
            :key="repo.path"
            class="repo-row"
            :class="{ active: repo.path === selectedPath, dirty: !repo.isClean, conflicted: repo.status.conflicted > 0 }"
            @click="selectedPath = repo.path"
          >
            <div class="repo-row-main">
              <span class="repo-name">{{ repo.name }}</span>
              <span class="repo-path">{{ repo.path }}</span>
            </div>
            <div class="repo-row-meta">
              <span class="branch-chip"><GitBranch :size="13" /> {{ repo.branch }}</span>
              <span v-if="repo.timings?.totalMs" class="pill timing" :title="scanTimingTitle(repo)">Scan {{ formatDurationMs(repo.timings?.totalMs) }}</span>
              <span v-if="repo.status.conflicted" class="pill danger">冲突 {{ repo.status.conflicted }}</span>
              <span v-if="!repo.hasUpstream" class="pill warn">无 upstream</span>
              <span v-if="repo.ahead" class="pill">领先 {{ repo.ahead }}</span>
              <span v-if="repo.behind" class="pill info">落后 {{ repo.behind }}</span>
              <span v-if="isPullProtected(repo)" class="pill warn">Pull 保护</span>
              <span v-if="changedCount(repo)" class="pill warn">{{ changedCount(repo) }} 改动</span>
              <span v-else class="pill ok">干净</span>
            </div>
          </button>

          <div v-if="!filteredRepositories.length" class="empty">
            {{ state.booting || state.scanning ? '扫描中' : '暂无仓库' }}
          </div>
        </div>
      </aside>

      <section class="diff-panel">
        <div v-if="selectedRepo" class="repo-toolbar">
          <div class="repo-title">
            <h2>{{ selectedRepo.name }}</h2>
            <p>{{ selectedRepo.path }}</p>
            <p v-if="selectedPullWarning" class="repo-action-note">{{ selectedPullWarning }}</p>
          </div>
          <div class="repo-actions">
            <button title="刷新仓库" :disabled="gitActionBusy" @click="refreshSelected">
              <RotateCw :size="17" :class="{ spin: state.scanning }" />
              刷新
            </button>
            <button title="Fetch selected: 更新远端引用，不会合并到本地工作区" :disabled="gitActionBusy" @click="updateRepositories('fetch', 'selected')">
              <Download :size="17" />
              Fetch
            </button>
            <button :title="selectedPullTitle" :disabled="gitActionBusy" @click="updateRepositories('pull', 'selected')">
              <GitPullRequest :size="17" />
              Pull
            </button>
          </div>
        </div>

        <div v-if="selectedRepo" class="repo-summary">
          <span><GitBranch :size="15" /> {{ selectedRepo.branch }}</span>
          <span>{{ selectedRepo.head }}</span>
          <span v-if="selectedRepo.status.conflicted" class="summary-danger">冲突 {{ selectedRepo.status.conflicted }}</span>
          <span v-else-if="!selectedRepo.isClean" class="summary-warn">{{ formatStatusBreakdown(selectedRepo.status) }}</span>
          <span v-if="selectedRepo.timings?.totalMs" :title="scanTimingTitle(selectedRepo)">Scan {{ formatDurationMs(selectedRepo.timings?.totalMs) }}</span>
          <span v-if="selectedRepo.upstream">{{ selectedRepo.upstream }}</span>
          <span v-else class="summary-warn">无 upstream</span>
          <span v-if="selectedRepo.lastCommit.hash">{{ selectedRepo.lastCommit.hash }} {{ selectedRepo.lastCommit.subject }}</span>
        </div>

        <div v-if="selectedRepo" class="diff-tabs">
          <button :class="{ active: activeDiffMode === 'working' }" @click="setDiffMode('working')">Working</button>
          <button :class="{ active: activeDiffMode === 'staged' }" @click="setDiffMode('staged')">Staged</button>
          <button :class="{ active: activeDiffMode === 'head' }" @click="setDiffMode('head')">HEAD</button>
        </div>

        <div v-if="selectedRepo && selectedFilePath" class="diff-target">
          <span>当前文件：{{ selectedFilePath }}<template v-if="selectedFile"> · {{ selectedFile.status }}</template></span>
          <div class="diff-target-actions">
            <button title="暂存当前文件" :disabled="gitActionBusy || !selectedFileCanStage" @click="stageSelectedFile">
              <Plus :size="15" />
              Stage
            </button>
            <button title="取消暂存当前文件" :disabled="gitActionBusy || !selectedFileCanUnstage" @click="unstageSelectedFile">
              <Minus :size="15" />
              Unstage
            </button>
            <button @click="clearSelectedFile">全部文件</button>
          </div>
        </div>

        <div class="diff-scroll">
          <div v-if="!selectedRepo" class="empty large">
            选择仓库查看 diff
          </div>
          <div v-else-if="state.loadingDiff" class="empty large">
            加载 diff
          </div>
          <div v-else-if="diff?.error" class="empty large danger-text">
            {{ diff.error }}
          </div>
          <div v-else-if="!renderedDiff?.files.length" class="empty large">
            {{ diff?.note || '当前视图没有 diff' }}
          </div>
          <template v-else>
            <div v-if="renderedDiff.truncated" class="diff-warning">
              Diff 内容较大，已截断显示。
            </div>
            <div v-if="renderedDiff.hiddenFiles || renderedDiff.hiddenLines" class="diff-warning">
              Diff 较大，当前渲染 {{ renderedDiff.files.length }} 个文件、{{ renderedDiff.renderedLines }} 行；仍有
              {{ renderedDiff.hiddenFiles }} 个文件、{{ renderedDiff.hiddenLines }} 行未渲染。
            </div>
            <div v-if="renderedDiff.note" class="diff-warning">
              {{ renderedDiff.note }}
            </div>

            <article v-for="file in renderedDiff.files" :key="`${file.oldPath}-${file.newPath}`" class="diff-file">
              <header>
                <div>
                  <strong>{{ file.newPath || file.oldPath }}</strong>
                  <span>{{ file.status }}</span>
                </div>
                <div class="diff-counts">
                  <span class="add">+{{ file.additions }}</span>
                  <span class="delete">-{{ file.deletions }}</span>
                </div>
              </header>

              <div class="diff-lines">
                <div
                  v-for="(line, index) in file.lines"
                  :key="`${file.newPath}-${index}`"
                  class="diff-line"
                  :class="line.kind"
                >
                  <span class="line-no">{{ line.oldLine || '' }}</span>
                  <span class="line-no">{{ line.newLine || '' }}</span>
                  <code>{{ line.content }}</code>
                </div>
              </div>
            </article>
          </template>
        </div>
      </section>

      <aside class="detail-panel">
        <section v-if="selectedRepo" class="side-section repo-advisory-section">
          <div class="side-head">
            <h2>状态建议</h2>
            <AlertTriangle v-if="selectedRepoHasDanger" :size="17" />
            <Check v-else :size="17" />
          </div>

          <div class="advisory-list">
            <div v-for="item in selectedRepoAdvisories" :key="`${item.label}-${item.detail}`" class="advisory-row" :class="item.tone">
              <strong>{{ item.label }}</strong>
              <small>{{ item.detail }}</small>
            </div>
          </div>
        </section>

        <section class="side-section">
          <div class="side-head">
            <h2>分支</h2>
            <GitBranch :size="17" />
          </div>

          <div v-if="state.loadingBranches" class="empty compact">加载分支</div>
          <template v-else>
            <div class="branch-group">
              <button
                v-for="branch in localBranches"
                :key="branch.name"
                class="branch-row"
                :class="{ current: branch.current }"
                :title="branchCheckoutTitle(branch)"
                :disabled="branch.current || Boolean(state.checkingOut) || gitActionBusy"
                @click="checkoutBranch(branch)"
              >
                <span>{{ branch.name }}</span>
                <small>{{ branch.commit }} {{ branch.relativeTime }}</small>
                <ChevronRight v-if="!branch.current" :size="15" />
                <Check v-else :size="15" />
              </button>
            </div>

            <div class="remote-list">
              <h3>远端</h3>
              <div v-for="branch in remoteBranches" :key="branch.name" class="remote-row">
                <span>{{ branch.name }}</span>
                <small>{{ branch.commit }}</small>
              </div>
            </div>
          </template>
        </section>

        <section class="side-section files-section">
          <div class="side-head">
            <h2>变更文件</h2>
            <span>
              {{ filteredSelectedFiles.length }}
              <template v-if="changedFileKeyword">/ {{ selectedFiles.length }}</template>
            </span>
          </div>

          <div v-if="!selectedFiles.length" class="empty compact">无变更</div>
          <div v-else>
            <div class="filter file-filter">
              <Search :size="15" />
              <input v-model="changedFileFilter" type="search" placeholder="过滤文件" />
            </div>
            <div v-if="!filteredSelectedFiles.length" class="empty compact">无匹配文件</div>
            <div v-else class="file-list">
              <button
                v-for="file in renderedSelectedFiles"
                :key="`${file.status}-${file.path}`"
                class="file-row"
                :class="{ active: file.path === selectedFilePath }"
                @click="selectChangedFile(file)"
              >
                <span class="file-status" :class="{ staged: file.staged }">{{ file.status }}</span>
                <span class="file-name">{{ file.path }}</span>
              </button>
              <div v-if="hiddenByFileFilterCount" class="list-limit-note">
                已过滤 {{ hiddenByFileFilterCount }} 个变更文件。
              </div>
              <div v-if="hiddenSelectedFileCount" class="list-limit-note">
                还有 {{ hiddenSelectedFileCount }} 个匹配文件未渲染。
              </div>
            </div>
          </div>
        </section>

        <section class="side-section updates-section">
          <div class="side-head">
            <h2>最近更新</h2>
            <RefreshCw :size="16" />
          </div>
          <div v-if="!updateResults.length" class="empty compact">暂无记录</div>
          <div v-else class="update-list">
            <div v-for="result in updateResults.slice(0, 8)" :key="`${result.path}-${result.finishedAt}`" class="update-row">
              <span :class="{ ok: result.success, warn: result.skipped, fail: !result.success && !result.skipped }">
                {{ result.success ? '成功' : result.skipped ? '跳过' : '失败' }}
              </span>
              <strong>{{ result.path.split(/[\\/]/).pop() }}</strong>
              <small>{{ result.message }}</small>
            </div>
          </div>
        </section>
      </aside>
    </div>
  </main>
</template>
