const PING_TIMEOUT_MS = 3_000
const PROBE_INTERVAL_MS = 60_000

function normalizeURL(value, variableName) {
  const text = String(value || '').trim()
  if (!text) return null

  let url
  try {
    url = new URL(text)
  } catch {
    throw new Error(`${variableName} 格式无效`)
  }
  const isLocal = url.hostname === '127.0.0.1' || url.hostname === 'localhost'
  if (url.protocol !== 'https:' && !isLocal) {
    throw new Error(`${variableName} 必须使用 HTTPS`)
  }
  url.pathname = url.pathname.replace(/\/$/, '')
  url.search = ''
  url.hash = ''
  return url
}

export function configuredAdminRoutes(env) {
  const legacyURL = String(env.ADMIN_SERVER_URL || '').trim()
  const values = [
    ['line1', env.ADMIN_SERVER_URL_LINE1 || legacyURL, 'ADMIN_SERVER_URL_LINE1'],
    ['line2', env.ADMIN_SERVER_URL_LINE2, 'ADMIN_SERVER_URL_LINE2'],
  ]
  const seen = new Set()
  return values.flatMap(([id, value, variableName]) => {
    const url = normalizeURL(value, variableName)
    if (!url || seen.has(url.toString())) return []
    seen.add(url.toString())
    return [{ id, url }]
  })
}

export class AdminRouteManager {
  constructor(routes, { fetchImpl = fetch, now = () => Date.now(), probeIntervalMs = PROBE_INTERVAL_MS } = {}) {
    if (!Array.isArray(routes) || routes.length === 0) throw new Error('至少配置一条远程管理线路')
    this.routes = routes
    this.fetchImpl = fetchImpl
    this.now = now
    this.probeIntervalMs = probeIntervalMs
    this.mode = 'automatic'
    this.currentRoute = routes[0]
    this.lastProbeAt = 0
    this.probePromise = null
  }

  async routeForRequest() {
    if (this.mode !== 'automatic') return this.currentRoute
    if (this.lastProbeAt === 0) {
      await this.refresh()
    } else if (this.now() - this.lastProbeAt >= this.probeIntervalMs) {
      void this.refresh()
    }
    return this.currentRoute
  }

  async refresh() {
    if (this.mode !== 'automatic') return this.currentRoute
    if (this.probePromise) return this.probePromise
    this.probePromise = this.performRefresh().finally(() => {
      this.probePromise = null
    })
    return this.probePromise
  }

  markSuccess(routeID) {
    const route = this.routes.find((candidate) => candidate.id === routeID)
    if (route) this.currentRoute = route
  }

  markFailure(routeID) {
    if (this.mode !== 'automatic') return
    if (this.currentRoute.id !== routeID) return
    const fallback = this.routes.find((candidate) => candidate.id !== routeID)
    if (fallback) this.currentRoute = fallback
  }

  alternateRoute(routeID) {
    if (this.mode !== 'automatic') return null
    return this.routes.find((candidate) => candidate.id !== routeID) || null
  }

  async setMode(mode) {
    if (mode === 'automatic') {
      this.mode = mode
      this.lastProbeAt = 0
      await this.refresh()
      return this.status()
    }

    const route = this.routes.find((candidate) => candidate.id === mode)
    if (!route) throw new Error('指定线路未配置')
    this.mode = mode
    this.currentRoute = route
    return this.status()
  }

  status() {
    return {
      mode: this.routes.length > 1 ? this.mode : 'single',
      current_route: this.currentRoute.id,
      routes: this.routes.map((route) => ({ id: route.id, url: route.url.toString() })),
    }
  }

  async performRefresh() {
    this.lastProbeAt = this.now()
    if (this.routes.length < 2) return this.currentRoute

    const results = await Promise.all(this.routes.map(async (route) => {
      const startedAt = this.now()
      try {
        const response = await this.fetchImpl(new URL('/ping', route.url), {
          method: 'GET',
          cache: 'no-store',
          signal: AbortSignal.timeout(PING_TIMEOUT_MS),
        })
        if (!response.ok) return null
        return { route, elapsed: this.now() - startedAt }
      } catch {
        return null
      }
    }))
    const reachable = results.filter(Boolean)
    if (reachable.length === 0) return this.currentRoute
    const fastest = reachable.reduce((best, result) => result.elapsed < best.elapsed ? result : best)
    this.currentRoute = fastest.route
    return this.currentRoute
  }
}

export function isRetryableRouteResponse(status) {
  return status === 502 || status === 503 || status === 504
}
