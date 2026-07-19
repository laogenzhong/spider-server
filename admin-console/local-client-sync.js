import { execFile } from 'node:child_process'
import { mkdtemp, readFile, readdir, rm, stat, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { isAbsolute, join, resolve } from 'node:path'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)
const MAX_SYNC_BODY_BYTES = 32 * 1024 * 1024

function sendJSON(res, status, payload) {
  res.statusCode = status
  res.setHeader('Content-Type', 'application/json; charset=utf-8')
  res.setHeader('Cache-Control', 'no-store')
  res.end(JSON.stringify(payload))
}

function isAllowedOrigin(origin) {
  if (!origin) return true
  try {
    const value = new URL(origin)
    return value.protocol === 'http:' && (value.hostname === '127.0.0.1' || value.hostname === 'localhost')
  } catch {
    return false
  }
}

async function readJSONBody(req) {
  const chunks = []
  let size = 0
  for await (const chunk of req) {
    size += chunk.length
    if (size > MAX_SYNC_BODY_BYTES) throw Object.assign(new Error('配置超过 32 MB'), { status: 413 })
    chunks.push(chunk)
  }
  try {
    return JSON.parse(Buffer.concat(chunks).toString('utf8'))
  } catch {
    throw Object.assign(new Error('请求必须是有效 JSON'), { status: 400 })
  }
}

async function requiredDirectory(path, label) {
  if (!path || !isAbsolute(path)) throw Object.assign(new Error(`${label}必须配置为绝对路径`), { status: 503 })
  try {
    const info = await stat(path)
    if (!info.isDirectory()) throw new Error('not directory')
  } catch {
    throw Object.assign(new Error(`${label}不存在：${path}`), { status: 503 })
  }
  return path
}

async function readCurrentWorkoutExploreConfig(clientRoot) {
  const configDirectory = join(clientRoot, 'WorkoutExploreConfigs')
  let filenames
  try {
    filenames = (await readdir(configDirectory))
      .filter(name => name.endsWith('.workout-explore.json'))
      .sort()
  } catch {
    throw Object.assign(new Error(`客户端探索配置目录不存在：${configDirectory}`), { status: 404 })
  }
  if (!filenames.length) {
    throw Object.assign(new Error('客户端当前没有探索配置文件'), { status: 404 })
  }
  if (filenames.length > 1) {
    throw Object.assign(new Error(`客户端存在 ${filenames.length} 份探索配置，暂不能确定要加载哪一份`), { status: 409 })
  }
  const filename = filenames[0]
  try {
    return {
      filename,
      document: JSON.parse(await readFile(join(configDirectory, filename), 'utf8')),
    }
  } catch {
    throw Object.assign(new Error(`无法读取客户端探索配置：${filename}`), { status: 422 })
  }
}

async function runNodeScript(scriptPath, args, cwd, environment = {}) {
  try {
    const result = await execFileAsync(process.execPath, [scriptPath, ...args], {
      cwd,
      env: { ...process.env, ...environment },
      timeout: 120000,
      maxBuffer: 16 * 1024 * 1024,
    })
    return [result.stdout, result.stderr].map((value) => value?.trim()).filter(Boolean).join('\n')
  } catch (error) {
    const detail = [error.stdout, error.stderr, error.message].map((value) => String(value || '').trim()).find(Boolean)
    throw Object.assign(new Error(detail || '客户端同步脚本执行失败'), { status: 422 })
  }
}

export function createLocalClientSyncPlugin(env) {
  const configuredClientRoot = (env.SPIDER_CLIENT_ROOT || '').trim()
  const configuredGIFRoot = (env.EXERCISE_GIF_ROOT || '').trim()
  let inFlight = false

  const attachMiddleware = (server) => {
    server.middlewares.use(async (req, res, next) => {
      const pathname = new URL(req.url || '/', 'http://127.0.0.1').pathname
      if (!pathname.startsWith('/local-client-sync/')) {
        next()
        return
      }
      if (!isAllowedOrigin(req.headers.origin)) {
        sendJSON(res, 403, { code: 403, message: '本地客户端同步服务拒绝了跨站请求' })
        return
      }
      if (req.method === 'GET' && pathname === '/local-client-sync/workout-explore') {
        try {
          const clientRoot = await requiredDirectory(configuredClientRoot ? resolve(configuredClientRoot) : '', 'SPIDER_CLIENT_ROOT')
          const current = await readCurrentWorkoutExploreConfig(clientRoot)
          sendJSON(res, 200, { code: 0, message: '客户端探索配置读取成功', ...current })
        } catch (error) {
          sendJSON(res, error.status || 500, { code: error.status || 500, message: error.message || '客户端探索配置读取失败' })
        }
        return
      }
      if (req.method !== 'POST') {
        sendJSON(res, 405, { code: 405, message: '只支持 POST；读取探索配置请使用 GET' })
        return
      }
      if (inFlight) {
        sendJSON(res, 409, { code: 409, message: '已有客户端同步任务正在执行' })
        return
      }

      let temporaryRoot = ''
      inFlight = true
      try {
        const clientRoot = await requiredDirectory(configuredClientRoot ? resolve(configuredClientRoot) : '', 'SPIDER_CLIENT_ROOT')
        const payload = await readJSONBody(req)
        temporaryRoot = await mkdtemp(join(tmpdir(), 'lifttags-client-sync-'))
        const temporaryConfig = join(temporaryRoot, 'config.json')
        await writeFile(temporaryConfig, `${JSON.stringify(payload, null, 2)}\n`, 'utf8')

        let output
        if (pathname === '/local-client-sync/exercise-library') {
          const gifRoot = await requiredDirectory(resolve(configuredGIFRoot || join(clientRoot, 'spider/Resources/ExerciseGIFs')), 'EXERCISE_GIF_ROOT')
          const scriptPath = resolve(clientRoot, 'scripts/import_exercise_library_package.mjs')
          output = await runNodeScript(scriptPath, ['--config', temporaryConfig, '--assets', gifRoot], clientRoot)
          const generatorPath = resolve(process.cwd(), 'scripts/generate-exercise-catalog.mjs')
          const refreshOutput = await runNodeScript(generatorPath, [], process.cwd(), {
            SPIDER_CLIENT_ROOT: clientRoot,
            EXERCISE_GIF_ROOT: gifRoot,
          })
          output = [output, refreshOutput].filter(Boolean).join('\n')
        } else if (pathname === '/local-client-sync/workout-explore') {
          const scriptPath = resolve(clientRoot, 'scripts/import_workout_explore_config.mjs')
          output = await runNodeScript(scriptPath, ['--config', temporaryConfig], clientRoot)
        } else {
          sendJSON(res, 404, { code: 404, message: '未知的客户端同步类型' })
          return
        }

        sendJSON(res, 200, { code: 0, message: '客户端配置更新成功', output })
      } catch (error) {
        sendJSON(res, error.status || 500, { code: error.status || 500, message: error.message || '客户端配置更新失败' })
      } finally {
        inFlight = false
        if (temporaryRoot) await rm(temporaryRoot, { recursive: true, force: true })
      }
    })
  }

  return {
    name: 'spider-local-client-sync',
    configureServer: attachMiddleware,
    configurePreviewServer: attachMiddleware,
  }
}
