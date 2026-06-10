import { cp, mkdir, readdir, readFile, rm, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'
import { compileScript, compileTemplate, parse } from 'vue/compiler-sfc'

const rootDir = join(dirname(fileURLToPath(import.meta.url)), '..')
const distDir = join(rootDir, 'dist')
const assetsDir = join(distDir, 'assets')

try {
  const { build } = await import('vite')
  await build({ clearScreen: false })
} catch (error) {
  if (!isNodeSpawnPolicyError(error)) {
    throw error
  }
  console.warn('[build] Vite is blocked by Node child_process spawn EPERM; using local fallback build.')
  await fallbackBuild()
}
await ensureDistGitkeep()

function isNodeSpawnPolicyError(error) {
  return String(error?.stack ?? error).includes('EPERM')
}

async function ensureDistGitkeep() {
  await mkdir(distDir, { recursive: true })
  await writeFileIfChanged(join(distDir, '.gitkeep'), '\n')
}

async function fallbackBuild() {
  try {
    await rm(distDir, { recursive: true, force: true })
  } catch (error) {
    if (error?.code !== 'EPERM') {
      throw error
    }
    console.warn(`[build] Could not fully clean dist because a file is locked: ${error.path}`)
    await cleanKnownGeneratedFiles()
  }
  await mkdir(join(assetsDir, 'api'), { recursive: true })
  await mkdir(join(assetsDir, 'vendor'), { recursive: true })
  await mkdir(join(distDir, 'wailsjs', 'go', 'main'), { recursive: true })

  await copyFileIfChanged(
    join(rootDir, 'wailsjs', 'go', 'main', 'App.js'),
    join(distDir, 'wailsjs', 'go', 'main', 'App.js'),
  )
  await copyFileIfChanged(
    join(rootDir, 'node_modules', 'vue', 'dist', 'vue.runtime.esm-browser.prod.js'),
    join(assetsDir, 'vendor', 'vue.runtime.esm-browser.prod.js'),
  )
  if (!(await pathExists(join(assetsDir, 'vendor', 'lucide', 'lucide-vue.mjs')))) {
    await cp(
      join(rootDir, 'node_modules', '@lucide', 'vue', 'dist', 'esm'),
      join(assetsDir, 'vendor', 'lucide'),
      { recursive: true, force: true },
    )
  }

  await writeFileIfChanged(join(assetsDir, 'style.css'), await readFile(join(rootDir, 'src', 'style.css'), 'utf8'))
  await writeFileIfChanged(join(assetsDir, 'main.js'), await buildMainModule())
  await writeFileIfChanged(join(assetsDir, 'App.js'), await buildVueModule())
  await writeFileIfChanged(join(assetsDir, 'api', 'backend.js'), await buildApiModule())
  await writeFileIfChanged(join(distDir, 'index.html'), buildHtml())
}

async function pathExists(path) {
  try {
    await readFile(path)
    return true
  } catch (error) {
    if (error?.code === 'ENOENT') {
      return false
    }
    throw error
  }
}

async function copyFileIfChanged(source, destination) {
  await writeFileIfChanged(destination, await readFile(source))
}

async function writeFileIfChanged(path, content) {
  try {
    const current = await readFile(path)
    const next = Buffer.isBuffer(content) ? content : Buffer.from(content)
    if (current.equals(next)) {
      return
    }
  } catch (error) {
    if (error?.code !== 'ENOENT') {
      throw error
    }
  }
  await writeFile(path, content)
}

async function cleanKnownGeneratedFiles() {
  let entries = []
  try {
    entries = await readdir(assetsDir, { withFileTypes: true })
  } catch (error) {
    if (error?.code !== 'ENOENT') {
      throw error
    }
    return
  }

  for (const entry of entries) {
    if (entry.isFile() && /^index-[\w-]+\.(?:js|css)$/.test(entry.name)) {
      try {
        await rm(join(assetsDir, entry.name), { force: true })
      } catch (error) {
        if (error?.code !== 'EPERM') {
          throw error
        }
        console.warn(`[build] Leaving locked stale asset in place: ${join(assetsDir, entry.name)}`)
      }
    }
  }
}

async function buildMainModule() {
  const source = await readFile(join(rootDir, 'src', 'main.ts'), 'utf8')
  return transpile(
    source
      .replace(/^import ['"]\.\/style\.css['"];?\r?\n/m, '')
      .replace(/from ['"]\.\/App\.vue['"]/g, "from './App.js'"),
  )
}

async function buildApiModule() {
  const source = await readFile(join(rootDir, 'src', 'api', 'backend.ts'), 'utf8')
  return addJsExtensions(transpile(source))
}

async function buildVueModule() {
  const source = await readFile(join(rootDir, 'src', 'App.vue'), 'utf8')
  const { descriptor, errors } = parse(source, { filename: 'App.vue' })
  if (errors.length) {
    throw new Error(errors.map((error) => String(error)).join('\n'))
  }

  const script = compileScript(descriptor, { id: 'fusion-git-desk-app' })
  const template = compileTemplate({
    source: descriptor.template?.content ?? '',
    filename: 'App.vue',
    id: 'fusion-git-desk-app',
    compilerOptions: {
      mode: 'module',
      bindingMetadata: script.bindings,
    },
  })
  if (template.errors.length) {
    throw new Error(template.errors.map((error) => String(error)).join('\n'))
  }

  const moduleSource = [
    script.content.replace(/export default\s+/, 'const __sfc__ = '),
    template.code.replace(/export function render/, 'function render'),
    '__sfc__.render = render',
    'export default __sfc__',
  ].join('\n\n')

  return addJsExtensions(transpile(moduleSource))
}

function transpile(source) {
  return ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ES2022,
      sourceMap: false,
    },
  }).outputText
}

function addJsExtensions(source) {
  return source
    .replace(/from ['"]\.\/api\/backend['"]/g, "from './api/backend.js'")
    .replace(/from ['"]\.\.\/\.\.\/wailsjs\/go\/main\/App['"]/g, "from '../../wailsjs/go/main/App.js'")
}

function buildHtml() {
  return `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Fusion Git Desk</title>
    <link rel="stylesheet" href="./assets/style.css" />
    <script type="importmap">
      {
        "imports": {
          "vue": "./assets/vendor/vue.runtime.esm-browser.prod.js",
          "@lucide/vue": "./assets/vendor/lucide/lucide-vue.mjs"
        }
      }
    </script>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="./assets/main.js"></script>
  </body>
</html>
`
}
