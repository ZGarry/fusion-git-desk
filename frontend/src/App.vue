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
  Pause,
  Play,
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
  error: '',
  notice: '',
  lastScan: '',
})

const rootInput = ref('')
const repoFilter = ref('')
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
  const stats = { total: repositories.value.length, dirty: 0, behind: 0, ahead: 0 }
  for (const repo of repositories.value) {
    if (!repo.isClean) stats.dirty++
    if (repo.behind > 0) stats.behind++
    if (repo.ahead > 0) stats.ahead++
  }
  return stats
})

const selectedFiles = computed(() => selectedRepo.value?.status.files ?? [])
const selectedFile = computed(() => selectedFiles.value.find((file) => file.path === selectedFilePath.value) ?? null)
const renderedSelectedFiles = computed(() => selectedFiles.value.slice(0, maxRenderedChangedFiles))
const hiddenSelectedFileCount = computed(() => Math.max(0, selectedFiles.value.length - renderedSelectedFiles.value.length))
const localBranches = computed(() => (branches.value?.branches ?? []).filter((branch) => !branch.remote))
const remoteBranches = computed(() => (branches.value?.branches ?? []).filter((branch) => branch.remote))
const renderedDiff = computed(() => buildRenderedDiff(diff.value))

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
  void loadSelectedDetails()
})

watch(activeDiffMode, () => {
  void loadDiff()
})

watch(
  () => [settings.autoRefresh, settings.refreshIntervalSeconds] as const,
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
    return
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
  } catch (error) {
    state.error = messageOf(error)
  } finally {
    state.scanning = false
  }
}

async function refreshSelected() {
  if (!selectedRepo.value) return
  state.error = ''
  state.scanning = true
  try {
    const repo = await api.refreshRepository(selectedRepo.value.path)
    replaceRepository(repo)
    selectedPath.value = repo.path
    void loadSelectedDetails()
    state.notice = `${repo.name} 已刷新`
  } catch (error) {
    state.error = messageOf(error)
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

  if (paths.length === 0) return

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
  } catch (error) {
    state.error = messageOf(error)
  } finally {
    state.updating = false
  }
}

async function checkoutBranch(branch: BranchInfo) {
  if (!selectedRepo.value || branch.remote || branch.current) return

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

function setupRefreshTimer() {
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
    refreshTimer = undefined
  }

  if (!settings.autoRefresh) return
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
  if (!rootInput.value.trim() || state.scanning || state.updating) return

  if (settings.autoPullCleanRepos) {
    await updateRepositories('pull', 'all', true)
    return
  }
  if (settings.autoFetch) {
    await updateRepositories('fetch', 'all', true)
    return
  }
  await scanRepositories(false)
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
  const success = results.filter((result) => result.success).length
  const skipped = results.filter((result) => result.skipped).length
  const failed = results.length - success - skipped
  const verb = mode === 'pull' ? '拉取' : '获取'
  return `${verb}完成：成功 ${success}，跳过 ${skipped}，失败 ${failed}`
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
        <button class="primary-button" :disabled="state.scanning" @click="scanRepositories(true)">
          <RefreshCw :size="17" :class="{ spin: state.scanning }" />
          扫描
        </button>
      </div>

      <div class="top-actions">
        <button title="Fetch all" :disabled="state.updating || repositories.length === 0" @click="updateRepositories('fetch', 'all')">
          <Download :size="17" />
          全部 Fetch
        </button>
        <button title="Pull clean repositories" :disabled="state.updating || repositories.length === 0" @click="updateRepositories('pull', 'all')">
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
      <div class="metric info">
        <span>{{ repoStats.behind }}</span>
        落后
      </div>
      <div class="metric">
        <span>{{ repoStats.ahead }}</span>
        领先
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
        {{ state.lastScan ? `上次扫描 ${formatDate(state.lastScan)}` : '未扫描' }}
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
            :class="{ active: repo.path === selectedPath, dirty: !repo.isClean }"
            @click="selectedPath = repo.path"
          >
            <div class="repo-row-main">
              <span class="repo-name">{{ repo.name }}</span>
              <span class="repo-path">{{ repo.path }}</span>
            </div>
            <div class="repo-row-meta">
              <span class="branch-chip"><GitBranch :size="13" /> {{ repo.branch }}</span>
              <span v-if="repo.ahead" class="pill">领先 {{ repo.ahead }}</span>
              <span v-if="repo.behind" class="pill info">落后 {{ repo.behind }}</span>
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
          </div>
          <div class="repo-actions">
            <button title="刷新仓库" :disabled="state.scanning" @click="refreshSelected">
              <RotateCw :size="17" :class="{ spin: state.scanning }" />
              刷新
            </button>
            <button title="Fetch selected" :disabled="state.updating" @click="updateRepositories('fetch', 'selected')">
              <Download :size="17" />
              Fetch
            </button>
            <button title="Pull selected" :disabled="state.updating" @click="updateRepositories('pull', 'selected')">
              <GitPullRequest :size="17" />
              Pull
            </button>
          </div>
        </div>

        <div v-if="selectedRepo" class="repo-summary">
          <span><GitBranch :size="15" /> {{ selectedRepo.branch }}</span>
          <span>{{ selectedRepo.head }}</span>
          <span v-if="selectedRepo.upstream">{{ selectedRepo.upstream }}</span>
          <span v-if="selectedRepo.lastCommit.hash">{{ selectedRepo.lastCommit.hash }} {{ selectedRepo.lastCommit.subject }}</span>
        </div>

        <div v-if="selectedRepo" class="diff-tabs">
          <button :class="{ active: activeDiffMode === 'working' }" @click="setDiffMode('working')">Working</button>
          <button :class="{ active: activeDiffMode === 'staged' }" @click="setDiffMode('staged')">Staged</button>
          <button :class="{ active: activeDiffMode === 'head' }" @click="setDiffMode('head')">HEAD</button>
        </div>

        <div v-if="selectedRepo && selectedFilePath" class="diff-target">
          <span>当前文件：{{ selectedFilePath }}<template v-if="selectedFile"> · {{ selectedFile.status }}</template></span>
          <button @click="clearSelectedFile">全部文件</button>
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
                :disabled="branch.current || Boolean(state.checkingOut)"
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
            <span>{{ selectedFiles.length }}</span>
          </div>

          <div v-if="!selectedFiles.length" class="empty compact">无变更</div>
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
            <div v-if="hiddenSelectedFileCount" class="list-limit-note">
              还有 {{ hiddenSelectedFileCount }} 个变更文件未渲染。
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
