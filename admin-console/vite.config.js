import crypto from 'node:crypto'
import { createReadStream } from 'node:fs'
import { stat } from 'node:fs/promises'
import { join, resolve, sep } from 'node:path'
import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { createLocalClientSyncPlugin } from './local-client-sync.js'
import { createLocalOfferReplyPlugin } from './local-offer-reply.js'

const MAX_BODY_BYTES = 1024 * 1024

function writeJSON(res, status, message) {
  res.statusCode = status
  res.setHeader('Content-Type', 'application/json; charset=utf-8')
  res.end(JSON.stringify({ code: status, message }))
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

function canonicalSignature(secret, method, requestURI, timestamp, nonce, body) {
  const bodyHash = crypto.createHash('sha256').update(body).digest('hex')
  const canonical = [method.toUpperCase(), requestURI, timestamp, nonce, bodyHash].join('\n')
  return crypto.createHmac('sha256', secret).update(canonical).digest('hex')
}

function adminAPIPlugin(env) {
  const serverURLText = (env.ADMIN_SERVER_URL || '').trim()
  const secret = (env.ADMIN_CONSOLE_SECRET || '').trim()

  return {
    name: 'spider-admin-secure-proxy',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        if (!req.url?.startsWith('/admin-api')) {
          next()
          return
        }
        if (!isAllowedOrigin(req.headers.origin)) {
          writeJSON(res, 403, '本地管理代理拒绝了跨站请求')
          return
        }
        if (!serverURLText || secret.length < 32) {
          writeJSON(res, 503, '请先在 admin-console/.env.local 配置远程地址和管理密钥')
          return
        }

        let serverURL
        try {
          serverURL = new URL(serverURLText)
        } catch {
          writeJSON(res, 503, 'ADMIN_SERVER_URL 格式无效')
          return
        }
        const isLocalRemote = serverURL.hostname === '127.0.0.1' || serverURL.hostname === 'localhost'
        if (serverURL.protocol !== 'https:' && !isLocalRemote) {
          writeJSON(res, 503, '远程管理接口必须使用 HTTPS')
          return
        }

        const chunks = []
        let size = 0
        try {
          for await (const chunk of req) {
            size += chunk.length
            if (size > MAX_BODY_BYTES) {
              writeJSON(res, 413, '请求体过大')
              return
            }
            chunks.push(chunk)
          }
        } catch {
          writeJSON(res, 400, '无法读取本地请求')
          return
        }

        const body = Buffer.concat(chunks)
        const incomingURL = new URL(req.url, 'http://127.0.0.1')
        const remotePath = incomingURL.pathname.replace(/^\/admin-api/, '/admin-console') + incomingURL.search
        const remoteURL = new URL(remotePath, serverURL)
        const timestamp = String(Math.floor(Date.now() / 1000))
        const nonce = crypto.randomBytes(24).toString('hex')
        const signature = canonicalSignature(secret, req.method || 'GET', remoteURL.pathname + remoteURL.search, timestamp, nonce, body)

        const headers = {
          Accept: 'application/json',
          'X-Admin-Timestamp': timestamp,
          'X-Admin-Nonce': nonce,
          'X-Admin-Signature': signature,
        }
        if (body.length > 0) {
          headers['Content-Type'] = req.headers['content-type'] || 'application/json'
        }

        try {
          const response = await fetch(remoteURL, {
            method: req.method,
            headers,
            body: body.length > 0 ? body : undefined,
            signal: AbortSignal.timeout(15000),
          })
          const responseBody = Buffer.from(await response.arrayBuffer())
          res.statusCode = response.status
          res.setHeader('Content-Type', response.headers.get('content-type') || 'application/json; charset=utf-8')
          res.setHeader('Cache-Control', 'no-store')
          res.end(responseBody)
        } catch (error) {
          writeJSON(res, 502, error?.name === 'TimeoutError' ? '远程服务响应超时' : '无法连接远程管理接口')
        }
      })
    },
  }
}

function localExerciseGIFPlugin(env) {
  const clientRoot = (env.SPIDER_CLIENT_ROOT || '').trim()
  const configuredRoot = (env.EXERCISE_GIF_ROOT || '').trim()
  const root = resolve(configuredRoot || join(clientRoot, 'spider/Resources/ExerciseGIFs'))

  const attachMiddleware = (server) => {
    server.middlewares.use(async (req, res, next) => {
      const pathname = new URL(req.url || '/', 'http://127.0.0.1').pathname
      if (!pathname.startsWith('/exercise-gifs/')) {
        next()
        return
      }

      let relativePath
      try {
        relativePath = decodeURIComponent(pathname.slice('/exercise-gifs/'.length))
      } catch {
        writeJSON(res, 400, '动图路径无效')
        return
      }
      const filePath = resolve(root, relativePath)
      if (!relativePath.toLocaleLowerCase().endsWith('.gif') || !filePath.startsWith(`${root}${sep}`)) {
        writeJSON(res, 403, '禁止访问该文件')
        return
      }

      try {
        const info = await stat(filePath)
        if (!info.isFile()) throw new Error('not a file')
        res.statusCode = 200
        res.setHeader('Content-Type', 'image/gif')
        res.setHeader('Content-Length', String(info.size))
        res.setHeader('Cache-Control', 'public, max-age=3600')
        if (req.method === 'HEAD') {
          res.end()
          return
        }
        createReadStream(filePath).pipe(res)
      } catch {
        writeJSON(res, 404, '未找到动作动图')
      }
    })
  }

  return {
    name: 'spider-local-exercise-gifs',
    configureServer: attachMiddleware,
    configurePreviewServer: attachMiddleware,
  }
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  return {
    plugins: [vue(), createLocalOfferReplyPlugin(env), localExerciseGIFPlugin(env), createLocalClientSyncPlugin(env), adminAPIPlugin(env)],
    server: {
      host: '127.0.0.1',
      port: 4178,
      strictPort: false,
    },
    preview: {
      host: '127.0.0.1',
      port: 4178,
    },
  }
})
