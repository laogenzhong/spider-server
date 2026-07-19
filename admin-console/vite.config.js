import crypto from 'node:crypto'
import { createReadStream } from 'node:fs'
import { stat } from 'node:fs/promises'
import { join, resolve, sep } from 'node:path'
import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { AdminRouteManager, configuredAdminRoutes, isRetryableRouteResponse } from './admin-route-manager.js'
import { createLocalClientSyncPlugin } from './local-client-sync.js'
import { createLocalOfferReplyPlugin } from './local-offer-reply.js'

const MAX_BODY_BYTES = 1024 * 1024

function writeJSON(res, status, message, data) {
  res.statusCode = status
  res.setHeader('Content-Type', 'application/json; charset=utf-8')
  res.end(JSON.stringify(data === undefined ? { code: status, message } : { code: 0, data }))
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
  const secret = (env.ADMIN_CONSOLE_SECRET || '').trim()
  let routeManager = null
  let routeConfigurationError = ''
  try {
    const routes = configuredAdminRoutes(env)
    if (routes.length === 0) throw new Error('请在 admin-console/.env.local 配置至少一条远程管理线路')
    routeManager = new AdminRouteManager(routes)
  } catch (error) {
    routeConfigurationError = error?.message || '远程管理线路配置无效'
  }

  async function forwardRequest(route, req, body, incomingURL) {
    const remotePath = incomingURL.pathname.replace(/^\/admin-api/, '/admin-console') + incomingURL.search
    const remoteURL = new URL(remotePath, route.url)
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

    const response = await fetch(remoteURL, {
      method: req.method,
      headers,
      body: body.length > 0 ? body : undefined,
      signal: AbortSignal.timeout(15000),
    })
    return {
      route,
      response,
      responseBody: Buffer.from(await response.arrayBuffer()),
    }
  }

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
        if (!routeManager || secret.length < 32) {
          writeJSON(res, 503, routeConfigurationError || '请先在 admin-console/.env.local 配置两条远程地址和管理密钥')
          return
        }

        const incomingURL = new URL(req.url, 'http://127.0.0.1')
        if (incomingURL.pathname === '/admin-api/route-status') {
          await routeManager.routeForRequest()
          writeJSON(res, 200, '', routeManager.status())
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
        if (incomingURL.pathname === '/admin-api/route-mode') {
          if ((req.method || 'GET').toUpperCase() !== 'PUT') {
            writeJSON(res, 405, '线路模式仅支持 PUT 请求')
            return
          }
          let mode
          try {
            mode = JSON.parse(body.toString('utf8')).mode
          } catch {
            writeJSON(res, 400, '线路模式请求无效')
            return
          }
          try {
            writeJSON(res, 200, '', await routeManager.setMode(mode))
          } catch (error) {
            writeJSON(res, 400, error?.message || '线路模式无效')
          }
          return
        }

        const isSafeToRetry = (req.method || 'GET').toUpperCase() === 'GET'
        const selectedRoute = await routeManager.routeForRequest()
        const attemptedRouteIDs = new Set([selectedRoute.id])
        let result

        try {
          result = await forwardRequest(selectedRoute, req, body, incomingURL)
        } catch (error) {
          routeManager.markFailure(selectedRoute.id)
          const fallbackRoute = isSafeToRetry ? routeManager.alternateRoute(selectedRoute.id) : null
          if (fallbackRoute) {
            try {
              attemptedRouteIDs.add(fallbackRoute.id)
              result = await forwardRequest(fallbackRoute, req, body, incomingURL)
              routeManager.markSuccess(fallbackRoute.id)
            } catch {
              routeManager.markFailure(fallbackRoute.id)
            }
          }
          if (!result) {
            writeJSON(res, 502, error?.name === 'TimeoutError' ? '两条远程线路均响应超时' : '无法连接远程管理线路')
            return
          }
        }

        if (isSafeToRetry && isRetryableRouteResponse(result.response.status)) {
          routeManager.markFailure(result.route.id)
          const fallbackRoute = routeManager.alternateRoute(result.route.id)
          if (fallbackRoute && !attemptedRouteIDs.has(fallbackRoute.id)) {
            try {
              attemptedRouteIDs.add(fallbackRoute.id)
              result = await forwardRequest(fallbackRoute, req, body, incomingURL)
              routeManager.markSuccess(fallbackRoute.id)
            } catch {
              routeManager.markFailure(fallbackRoute.id)
            }
          }
        } else if (!isRetryableRouteResponse(result.response.status)) {
          routeManager.markSuccess(result.route.id)
        }

        res.statusCode = result.response.status
        res.setHeader('Content-Type', result.response.headers.get('content-type') || 'application/json; charset=utf-8')
        res.setHeader('Cache-Control', 'no-store')
        res.setHeader('X-Admin-Route', result.route.id)
        res.end(result.responseBody)
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
