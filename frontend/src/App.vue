<script setup lang="ts">
import {
  AlertTriangle,
  Check,
  ChevronRight,
  Download,
  ExternalLink,
  FileSearch,
  FolderOpen,
  GitBranch,
  GitCompare,
  GitPullRequest,
  MoreHorizontal,
  RefreshCw,
  RotateCw,
  Search,
  X,
} from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { BrowserOpenURL } from '../wailsjs/runtime/runtime'
import { api } from './api/backend'
import type {
  BranchInfo,
  BranchResponse,
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
  ideaPath: '',
  diffDisplayByteLimit: 900000,
})

const state = reactive({
  hasGit: true,
  booting: true,
  scanning: false,
  updating: false,
  loadingBranches: false,
  checkingOut: '',
  openingEditor: '',
  error: '',
  notice: '',
  scanWarnings: [] as string[],
  lastScan: '',
  autoCycleNote: '自动刷新已暂停',
  autoCycleFailureCount: 0,
  autoCycleLastError: '',
  autoCycleFailureTimes: [] as number[],
})

const rootInput = ref('')
const repoFilter = ref('')
const repositories = ref<Repository[]>([])
const selectedPath = ref('')
const branches = ref<BranchResponse | null>(null)
const updateResults = ref<UpdateResult[]>([])
const remoteBranchMenu = ref('')
const remoteCheckoutCandidate = ref<BranchInfo | null>(null)

const autoCycleFailureWindowMs = 10 * 60 * 1000

let refreshTimer: number | undefined
let settingsSaveTimer: number | undefined
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
const selectedDeploymentUrl = computed(() => selectedRepo.value ? deploymentUrl(selectedRepo.value) : '')

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

const scanWarningMessage = computed(() => {
  if (!state.scanWarnings.length) return ''
  const firstWarning = state.scanWarnings[0]
  const more = state.scanWarnings.length > 1 ? `，另有 ${state.scanWarnings.length - 1} 个路径也未检查` : ''
  return `扫描完成，但有 ${state.scanWarnings.length} 个路径未能检查：${firstWarning}${more}`
})

const selectedPullWarning = computed(() => {
  if (!selectedRepo.value) return ''
  if (selectedRepo.value.status.conflicted > 0) {
    return '当前仓库存在冲突，建议先在编辑器里解决后再拉取或切换分支。'
  }
  if (!selectedRepo.value.hasUpstream) {
    return selectedRepo.value.hasRemote
      ? '仓库已有远程仓库，但当前分支未设置上游，拉取会跳过。'
      : '当前仓库没有远程仓库，拉取会跳过。'
  }
  if (selectedRepo.value.isClean || !settings.onlyPullCleanRepos) return ''
  return '当前仓库有本地改动，拉取会跳过；可以先用编辑器提交，或只检查远端状态。'
})
const selectedRepoAdvisories = computed(() => selectedRepo.value ? buildRepoAdvisories(selectedRepo.value) : [])
const gitActionBusy = computed(() => state.scanning || state.updating || Boolean(state.openingEditor))
const fetchAllTitle = computed(() => '全部检查远端：更新远端状态，不修改本地代码。')
const pullAllTitle = computed(() => summarizePullPreflight(repositories.value, '全部拉取代码'))
const selectedPullTitle = computed(() => selectedRepo.value ? summarizePullPreflight([selectedRepo.value], '拉取代码') : '拉取代码')
const localBranches = computed(() => (branches.value?.branches ?? []).filter((branch) => !branch.remote))
const remoteBranches = computed(() => (branches.value?.branches ?? []).filter((branch) => branch.remote && !isIgnoredRemoteBranch(branch.name)))
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
  remoteBranchMenu.value = ''
  remoteCheckoutCandidate.value = null
  void loadSelectedDetails()
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

async function chooseIdeaPath() {
  const picked = await api.pickIdeaExecutable()
  if (picked) {
    settings.ideaPath = picked
    await api.saveSettings({ ...settings })
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
  state.scanWarnings = []
  state.scanning = true
  try {
    const response = await api.scanRepositories(root, settings.maxDepth)
    repositories.value = response.repositories ?? []
    state.scanWarnings = response.warnings ?? []
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
    branches.value = null
    return
  }

  await loadBranches()
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
      remoteBranchMenu.value = ''
      if (remoteCheckoutCandidate.value && !nextBranches.branches.some((branch) => branch.name === remoteCheckoutCandidate.value?.name)) {
        remoteCheckoutCandidate.value = null
      }
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
      state.notice = `扫描或刷新进行中，已跳过${mode === 'pull' ? '拉取' : '检查远端'}请求`
    }
    return null
  }
  if (state.updating) {
    if (!silent) {
      state.notice = `已有批量更新进行中，已跳过${mode === 'pull' ? '拉取' : '检查远端'}请求`
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
    void loadBranches()
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

async function openSelectedRepository(editor: 'vscode' | 'idea') {
  if (!selectedRepo.value) return
  if (guardBusyAction('打开项目')) {
    return
  }

  const editorLabel = editor === 'vscode' ? 'VS Code' : 'IDEA'
  state.openingEditor = editor
  state.error = ''
  try {
    if (editor === 'idea') {
      await api.saveSettings({ ...settings })
    }
    const result = await api.openRepository(selectedRepo.value.path, editor)
    if (!result.success) {
      state.error = result.message
      return
    }
    state.notice = result.message || `已用 ${editorLabel} 打开 ${selectedRepo.value.name}`
  } catch (error) {
    state.error = messageOf(error)
  } finally {
    state.openingEditor = ''
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

async function checkoutRemoteBranch(branch: BranchInfo) {
  if (!selectedRepo.value || !branch.remote) return
  if (guardBusyAction('切换远端分支')) {
    return
  }

  state.checkingOut = branch.name
  remoteCheckoutCandidate.value = null
  remoteBranchMenu.value = ''
  state.error = ''
  try {
    const result = await api.checkoutRemoteBranch(selectedRepo.value.path, branch.name)
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

function toggleRemoteBranchMenu(branch: BranchInfo) {
  if (gitActionBusy.value || state.checkingOut) return
  remoteBranchMenu.value = remoteBranchMenu.value === branch.name ? '' : branch.name
}

function requestRemoteCheckout(branch: BranchInfo) {
  remoteBranchMenu.value = ''
  remoteCheckoutCandidate.value = branch
}

function cancelRemoteCheckout() {
  remoteCheckoutCandidate.value = null
}

async function confirmRemoteCheckout() {
  if (!remoteCheckoutCandidate.value) return
  await checkoutRemoteBranch(remoteCheckoutCandidate.value)
}

function openDeploymentPage() {
  const url = selectedDeploymentUrl.value
  if (!url) {
    state.notice = '未识别部署页：当前仓库没有可推导的 GitLab 远端地址'
    return
  }
  openExternalUrl(url)
  state.notice = `已打开部署页 ${url}`
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
      recordAutoCycleFailure(`${modeLabel}结束于 ${finishedAt}`, state.error || '自动检查远端未返回结果')
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
      detail: '先用编辑器解决冲突并刷新仓库，再拉取代码或切换分支。',
    })
  }
  if (!repo.hasUpstream) {
    advisories.push({
      tone: 'warn',
      label: repo.hasRemote ? '未设置上游分支' : '无远程仓库',
      detail: repo.hasRemote
        ? `仓库已配置远端 ${repo.remoteName || 'remote'}，但当前分支没有跟踪分支；可以检查远端状态，拉取会跳过。`
        : '当前仓库没有配置 remote；检查远端和拉取都没有可用目标。',
    })
  }
  if (repo.behind > 0 && repo.hasUpstream) {
    if (isPullProtected(repo)) {
      advisories.push({
        tone: 'warn',
        label: `落后 ${repo.behind} 且有本地改动`,
        detail: '当前保护策略会跳过拉取；可以先用编辑器提交改动，或只检查远端状态。',
      })
    } else if (repo.isClean) {
      advisories.push({
        tone: 'info',
        label: `可拉取 ${repo.behind}`,
        detail: '工作区没有本地改动，适合执行安全拉取。',
      })
    } else {
      advisories.push({
        tone: 'warn',
        label: `落后 ${repo.behind} 且工作区不干净`,
        detail: '已关闭保护策略，拉取前建议先用编辑器确认本地改动。',
      })
    }
  }
  if (repo.ahead > 0) {
    advisories.push({
      tone: 'info',
      label: `本地领先 ${repo.ahead}`,
      detail: '本地有提交尚未进入远端；推送能力后续应按单仓库或明确批量策略补齐。',
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
      detail: '当前分支与远端没有已知领先或落后，可以继续查看代码或检查远端状态。',
    })
  }

  return advisories
}

function formatStatusBreakdown(status: Repository['status']) {
  const parts = [
    status.conflicted > 0 ? `冲突 ${status.conflicted}` : '',
    status.modified > 0 ? `修改 ${status.modified}` : '',
    status.added > 0 ? `新增 ${status.added}` : '',
    status.deleted > 0 ? `删除 ${status.deleted}` : '',
    status.renamed > 0 ? `改名 ${status.renamed}` : '',
    status.untracked > 0 ? `新文件 ${status.untracked}` : '',
  ].filter(Boolean)
  return parts.length ? parts.join('，') : `${status.files.length} 个文件有改动`
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
    noUpstream > 0 ? `${noUpstream} 个未设置上游会跳过` : '',
    conflicted > 0 ? `${conflicted} 个有冲突会跳过` : '',
  ].filter(Boolean)

  return `${action}：${parts.length ? parts.join('；') : '执行安全拉取'}`
}

function branchCheckoutTitle(branch: BranchInfo) {
  if (branch.current) return '当前分支'
  if (selectedRepo.value?.status.conflicted) return '存在冲突，建议先解决冲突后再切换分支'
  if (selectedRepo.value && !selectedRepo.value.isClean) return '有本地改动，切换分支可能被 Git 拒绝以保护文件'
  return `切换到 ${branch.name}`
}

function missingUpstreamLabel(repo: Repository) {
  return repo.hasRemote ? '未设置上游' : '无远程仓库'
}

function remoteBranchCheckoutTitle(branch: BranchInfo) {
  if (selectedRepo.value?.status.conflicted) return '存在冲突，建议先解决冲突后再切换远端分支'
  if (selectedRepo.value && !selectedRepo.value.isClean) return '有本地改动，切换分支可能被 Git 拒绝以保护文件'
  return `拉取 ${branch.name} 并切换到 ${remoteBranchLocalName(branch.name)}`
}

function remoteBranchLocalName(name: string) {
  return name.replace(/^[^/]+\//, '')
}

function isIgnoredRemoteBranch(name: string) {
  const branchName = remoteBranchLocalName(name)
  return branchName === 'ai-review' || branchName.startsWith('ai-review/')
}

function deploymentUrl(repo: Repository) {
  const webUrl = repositoryWebUrl(repo.remoteUrl)
  return webUrl ? `${webUrl}/-/pipelines` : ''
}

function repositoryWebUrl(remoteUrl: string) {
  const value = remoteUrl.trim()
  if (!value) return ''

  if (/^https?:\/\//i.test(value)) {
    try {
      const url = new URL(value)
      url.username = ''
      url.password = ''
      url.search = ''
      url.hash = ''
      url.pathname = trimGitSuffix(url.pathname)
      return url.toString().replace(/\/$/, '')
    } catch {
      return trimGitSuffix(value)
    }
  }

  const sshUrl = value.match(/^ssh:\/\/(?:[^@/]+@)?([^/:]+)(?::\d+)?\/(.+)$/i)
  if (sshUrl) {
    return `${gitWebProtocol(sshUrl[1])}://${sshUrl[1]}/${trimGitSuffix(sshUrl[2])}`
  }

  const scpLike = value.match(/^(?:[^@]+@)?([^:]+):(.+)$/)
  if (scpLike && !/^[a-zA-Z]$/.test(scpLike[1]) && !scpLike[2].startsWith('\\')) {
    return `${gitWebProtocol(scpLike[1])}://${scpLike[1]}/${trimGitSuffix(scpLike[2])}`
  }

  return ''
}

function trimGitSuffix(value: string) {
  return value.replace(/\/+$/, '').replace(/\.git$/i, '')
}

function gitWebProtocol(host: string) {
  return host.toLowerCase() === 'git.fusionfintrade.com' ? 'http' : 'https'
}

function openExternalUrl(url: string) {
  const runtimeWindow = window as Window & { runtime?: { BrowserOpenURL?: (url: string) => void } }
  if (runtimeWindow.runtime?.BrowserOpenURL) {
    BrowserOpenURL(url)
    return
  }
  window.open(url, '_blank', 'noopener,noreferrer')
}

function guardBusyAction(action: string, showNotice = true) {
  let reason = ''
  if (state.scanning) {
    reason = '已有扫描或刷新进行中'
  } else if (state.updating) {
    reason = '批量更新进行中'
  } else if (state.openingEditor) {
    reason = '正在打开编辑器'
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
  if (settings.autoPullCleanRepos) return '自动拉取'
  if (settings.autoFetch) return '自动检查远端'
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
    ideaPath: value.ideaPath || '',
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
  const verb = mode === 'pull' ? '拉取' : '检查远端'
  return { success, skipped, failed, verb }
}

function summarizeUpdateDetail(results: UpdateResult[], mode: UpdateMode) {
  const skipped = results.filter((result) => result.skipped)
  const failed = results.filter((result) => !result.success && !result.skipped)
  if (skipped.length && mode === 'pull') {
    const dirtySkipped = skipped.filter((result) => isDirtyWorkingTreeSkip(result))
    if (dirtySkipped.length === skipped.length) {
      return skipped.length === 1
        ? '工作区有本地改动，已跳过拉取；可先检查远端状态'
        : `${skipped.length} 个仓库有本地改动，已跳过拉取；可先检查远端状态`
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
          全部检查远端
        </button>
        <button :title="pullAllTitle" :disabled="gitActionBusy || repositories.length === 0" @click="updateRepositories('pull', 'all')">
          <GitPullRequest :size="17" />
          全部拉取代码
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
        未设置上游
      </div>
      <div class="metric warn">
        <span>{{ repoStats.pullBlocked }}</span>
        拉取保护
      </div>

      <div class="toggles">
        <label>
          <input v-model="settings.autoRefresh" type="checkbox" />
          {{ settings.autoRefresh ? '自动刷新' : '手动刷新' }}
        </label>
        <label>
          <input v-model="settings.autoFetch" type="checkbox" />
          自动检查远端
        </label>
        <label>
          <input v-model="settings.autoPullCleanRepos" type="checkbox" />
          自动拉取
        </label>
        <label>
          间隔
          <input v-model.number="settings.refreshIntervalSeconds" class="small-number" min="15" max="3600" type="number" />
          秒
        </label>
        <label>
          <input v-model="settings.onlyPullCleanRepos" type="checkbox" />
          只在无本地改动时拉取
        </label>
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
    <section v-else-if="scanWarningMessage" class="banner warn">
      <AlertTriangle :size="18" />
      {{ scanWarningMessage }}
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
              <span v-if="repo.status.conflicted" class="pill danger">冲突 {{ repo.status.conflicted }}</span>
              <span v-if="!repo.hasUpstream" class="pill warn">{{ missingUpstreamLabel(repo) }}</span>
              <span v-if="repo.ahead" class="pill">领先 {{ repo.ahead }}</span>
              <span v-if="repo.behind" class="pill info">落后 {{ repo.behind }}</span>
              <span v-if="isPullProtected(repo)" class="pill warn">拉取保护</span>
              <span v-if="changedCount(repo)" class="pill warn">{{ changedCount(repo) }} 改动</span>
              <span v-else class="pill ok">干净</span>
            </div>
          </button>

          <div v-if="!filteredRepositories.length" class="empty">
            {{ state.booting || state.scanning ? '扫描中' : '暂无仓库' }}
          </div>
        </div>
      </aside>

      <section class="main-panel">
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
            <button title="检查远端状态，不修改本地代码" :disabled="gitActionBusy" @click="updateRepositories('fetch', 'selected')">
              <Download :size="17" />
              检查远端
            </button>
            <button :title="selectedPullTitle" :disabled="gitActionBusy" @click="updateRepositories('pull', 'selected')">
              <GitPullRequest :size="17" />
              拉取代码
            </button>
          </div>
        </div>

        <div v-if="selectedRepo" class="repo-summary">
          <span><GitBranch :size="15" /> {{ selectedRepo.branch }}</span>
          <span>{{ selectedRepo.head }}</span>
          <span v-if="selectedRepo.status.conflicted" class="summary-danger">冲突 {{ selectedRepo.status.conflicted }}</span>
          <span v-else-if="!selectedRepo.isClean" class="summary-warn">{{ formatStatusBreakdown(selectedRepo.status) }}</span>
          <span v-if="selectedRepo.upstream">上游 {{ selectedRepo.upstream }}</span>
          <span v-else class="summary-warn">{{ missingUpstreamLabel(selectedRepo) }}</span>
          <span v-if="selectedRepo.lastCommit.hash">最近提交 {{ selectedRepo.lastCommit.hash }} {{ selectedRepo.lastCommit.subject }}</span>
        </div>

        <div class="main-content">
          <div v-if="!selectedRepo" class="empty large">
            选择仓库查看状态和分支
          </div>
          <template v-else-if="selectedRepo">
            <section class="overview-section">
              <div class="overview-head">
                <h2>下一步</h2>
              </div>
              <div class="advisory-list">
                <div v-for="item in selectedRepoAdvisories" :key="`${item.label}-${item.detail}`" class="advisory-row" :class="item.tone">
                  <strong>{{ item.label }}</strong>
                  <small>{{ item.detail }}</small>
                </div>
              </div>
            </section>

            <section class="overview-section">
              <div class="overview-head">
                <h2>打开项目</h2>
              </div>
              <div class="open-actions">
                <button :disabled="gitActionBusy" @click="openSelectedRepository('vscode')">
                  <FolderOpen :size="17" />
                  用 VS Code 打开
                </button>
                <button :disabled="gitActionBusy" @click="openSelectedRepository('idea')">
                  <FolderOpen :size="17" />
                  用 IDEA 打开
                </button>
                <button :title="selectedDeploymentUrl || '未识别 GitLab 远端地址'" :disabled="!selectedDeploymentUrl" @click="openDeploymentPage">
                  <ExternalLink :size="17" />
                  打开部署页
                </button>
              </div>
              <div class="editor-setting">
                <label>
                  <span>IDEA 路径</span>
                  <input v-model="settings.ideaPath" spellcheck="false" placeholder="自动查找" />
                </label>
                <button title="选择 IDEA 启动程序" :disabled="gitActionBusy" @click="chooseIdeaPath">
                  <FileSearch :size="17" />
                  选择
                </button>
                <button v-if="settings.ideaPath" title="清空 IDEA 路径" :disabled="gitActionBusy" @click="settings.ideaPath = ''">
                  <X :size="17" />
                  清空
                </button>
              </div>
            </section>
          </template>
        </div>
      </section>

      <aside class="detail-panel">
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
              <template
                v-for="branch in remoteBranches"
                :key="branch.name"
              >
                <div class="remote-row" :class="{ default: branch.default }">
                  <span>{{ branch.name }}</span>
                  <small>
                    <em v-if="branch.default">默认</em>
                    {{ branch.commit }}
                  </small>
                  <button
                    class="remote-action-button"
                    :title="`打开 ${branch.name} 的操作`"
                    :disabled="Boolean(state.checkingOut) || gitActionBusy"
                    @click="toggleRemoteBranchMenu(branch)"
                  >
                    <MoreHorizontal :size="15" />
                  </button>
                </div>
                <div v-if="remoteBranchMenu === branch.name" class="remote-action-popover">
                  <strong>{{ remoteBranchLocalName(branch.name) }}</strong>
                  <small>{{ remoteBranchCheckoutTitle(branch) }}</small>
                  <button :disabled="Boolean(state.checkingOut) || gitActionBusy" @click="requestRemoteCheckout(branch)">
                    <GitPullRequest :size="15" />
                    拉取并切换
                  </button>
                </div>
              </template>
              <div v-if="!remoteBranches.length" class="empty compact">暂无远端分支</div>
            </div>
          </template>
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

    <div v-if="remoteCheckoutCandidate" class="modal-backdrop" @click.self="cancelRemoteCheckout">
      <section class="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="remote-checkout-title">
        <div class="confirm-head">
          <h2 id="remote-checkout-title">切换远程分支</h2>
          <button class="icon-button" title="关闭" :disabled="Boolean(state.checkingOut)" @click="cancelRemoteCheckout">
            <X :size="16" />
          </button>
        </div>
        <div class="confirm-body">
          <p>
            将先拉取 <strong>{{ remoteCheckoutCandidate.name }}</strong>，再切换到本地分支
            <strong>{{ remoteBranchLocalName(remoteCheckoutCandidate.name) }}</strong>。
          </p>
          <p v-if="selectedRepo && !selectedRepo.isClean" class="confirm-warning">
            当前仓库有本地改动，Git 可能会拒绝切换以保护文件。
          </p>
          <p v-if="selectedRepo?.status.conflicted" class="confirm-warning">
            当前仓库存在冲突，建议先解决冲突后再切换分支。
          </p>
        </div>
        <div class="confirm-actions">
          <button :disabled="Boolean(state.checkingOut)" @click="cancelRemoteCheckout">取消</button>
          <button class="primary-button" :disabled="Boolean(state.checkingOut) || gitActionBusy" @click="confirmRemoteCheckout">
            <GitPullRequest :size="16" />
            确认拉取并切换
          </button>
        </div>
      </section>
    </div>
  </main>
</template>
