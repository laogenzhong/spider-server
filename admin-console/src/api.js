const routeListeners = new Set()

export function subscribeAdminRoute(listener) {
  routeListeners.add(listener)
  return () => routeListeners.delete(listener)
}

export async function request(path, options = {}) {
  const response = await fetch(`/admin-api${path}`, {
    method: options.method || 'GET',
    headers: options.body ? { 'Content-Type': 'application/json' } : undefined,
    body: options.body ? JSON.stringify(options.body) : undefined,
  })
  const routeID = response.headers.get('X-Admin-Route')
  if (routeID) routeListeners.forEach((listener) => listener(routeID))
  let payload
  try {
    payload = await response.json()
  } catch {
    payload = { message: `请求失败 (${response.status})` }
  }
  if (!response.ok || payload.code !== 0) {
    throw new Error(payload.message || `请求失败 (${response.status})`)
  }
  return normalizeResponseData(payload.data)
}

export async function localOfferReplyRequest(path, options = {}) {
  const hasRawBody = options.rawBody !== undefined
  const response = await fetch(`/local-offer-reply${path}`, {
    method: options.method || 'GET',
    headers: options.headers || (options.body ? { 'Content-Type': 'application/json' } : undefined),
    body: hasRawBody ? options.rawBody : (options.body ? JSON.stringify(options.body) : undefined),
  })
  let payload
  try {
    payload = await response.json()
  } catch {
    payload = { message: `本地请求失败 (${response.status})` }
  }
  if (!response.ok || payload.code !== 0) {
    throw new Error(payload.message || `本地请求失败 (${response.status})`)
  }
  return normalizeResponseData(payload.data)
}

function normalizeResponseData(data) {
  if (data && typeof data === 'object' && 'items' in data && !Array.isArray(data.items)) {
    return { ...data, items: [] }
  }
  return data
}

export function queryString(values) {
  const params = new URLSearchParams()
  Object.entries(values).forEach(([key, value]) => {
    if (value !== '' && value !== null && value !== undefined) {
      params.set(key, String(value))
    }
  })
  const text = params.toString()
  return text ? `?${text}` : ''
}
