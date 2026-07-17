import crypto from 'node:crypto'
import { promises as fs } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { parse } from 'csv-parse/sync'

const STORE_VERSION = 1
const MAX_JSON_BODY_BYTES = 4096
const MAX_CSV_BODY_BYTES = 10 * 1024 * 1024

export class LocalOfferReplyInputError extends Error {}

export class LocalOfferReplyService {
  constructor({ storePath }) {
    this.storePath = storePath
    this.queue = Promise.resolve()
  }

  static async create({
    storePath = '',
    cwd = process.cwd(),
    homeDir = os.homedir(),
  } = {}) {
    const resolvedStorePath = storePath
      ? path.resolve(cwd, storePath)
      : path.join(homeDir, '.lifttags-admin', 'offer-codes.json')
    const service = new LocalOfferReplyService({ storePath: resolvedStorePath })
    try {
      await service.readStore()
      return service
    } catch (error) {
      if (error?.code !== 'ENOENT') throw error
    }
    await service.writeStore(createEmptyStore())
    return service
  }

  async status() {
    return statusFromStore(await this.readStore())
  }

  async list({ search = '', status = 'all', page = 1, pageSize = 50 } = {}) {
    const store = await this.readStore()
    const normalizedSearch = String(search).trim().toUpperCase()
    const normalizedStatus = ['used', 'unused'].includes(status) ? status : 'all'
    const safePageSize = Math.min(100, Math.max(1, Number(pageSize) || 50))
    const safePage = Math.max(1, Number(page) || 1)
    const filtered = store.codes.filter((item) => {
      if (normalizedStatus === 'used' && !item.used) return false
      if (normalizedStatus === 'unused' && item.used) return false
      if (!normalizedSearch) return true
      return String(item.id) === normalizedSearch
        || item.code.includes(normalizedSearch)
        || item.url.toUpperCase().includes(normalizedSearch)
    }).sort((left, right) => left.id - right.id)
    const offset = (safePage - 1) * safePageSize
    return {
      items: filtered.slice(offset, offset + safePageSize),
      total: filtered.length,
      page: safePage,
      page_size: safePageSize,
    }
  }

  async importCSV(data, fileName) {
    return this.mutate(async (store) => {
      const records = parseOfferCSV(data)
      const importedAt = new Date().toISOString()
      const safeFileName = path.basename(String(fileName || 'uploaded.csv'))
      const byCode = new Map(store.codes.map((item) => [item.code, item]))
      let nextID = store.codes.reduce((maximum, item) => Math.max(maximum, item.id), 0) + 1
      let addedCount = 0
      let updatedCount = 0
      let existingCount = 0

      for (const record of records) {
        const existing = byCode.get(record.code)
        if (existing) {
          existingCount += 1
          if (existing.url !== record.url) {
            existing.url = record.url
            existing.updated_at = importedAt
            updatedCount += 1
          }
          continue
        }
        const item = createStoredCode(nextID, record, safeFileName, importedAt, null)
        store.codes.push(item)
        byCode.set(item.code, item)
        nextID += 1
        addedCount += 1
      }
      store.codes.sort((left, right) => left.id - right.id)
      store.imports.push({
        file_name: safeFileName,
        imported_at: importedAt,
        added_count: addedCount,
        updated_count: updatedCount,
        existing_count: existingCount,
        migration: false,
      })
      store.imports = store.imports.slice(-100)
      return {
        result: {
          file_name: safeFileName,
          imported_count: records.length,
          added_count: addedCount,
          updated_count: updatedCount,
          existing_count: existingCount,
          status: statusFromStore(store),
        },
        changed: true,
      }
    })
  }

  async generate(input) {
    return this.mutate(async (store) => {
      const item = findStoredCode(String(input || '').trim(), store.codes)
      const alreadyUsed = item.used
      if (!alreadyUsed) markCodeUsage(item, true, 'reply_generated')
      const status = statusFromStore(store)
      return {
        result: {
          id: item.id,
          already_used: alreadyUsed,
          reply: buildReply(item),
          status,
        },
        changed: !alreadyUsed,
      }
    })
  }

  async setUsage(rangeText, used = true) {
    return this.mutate(async (store) => {
      const ids = parseSequenceRange(rangeText)
      const byID = new Map(store.codes.map((item) => [item.id, item]))
      const missingID = ids.find((id) => !byID.has(id))
      if (missingID) throw new LocalOfferReplyInputError(`兑换序号 ${missingID} 不存在`)
      let changedCount = 0
      for (const id of ids) {
        const item = byID.get(id)
        if (item.used === used) continue
        markCodeUsage(item, used, used ? 'manual_batch' : null)
        changedCount += 1
      }
      return {
        result: {
          selected_count: ids.length,
          changed_count: changedCount,
          used,
          status: statusFromStore(store),
        },
        changed: changedCount > 0,
      }
    })
  }

  async mutate(action) {
    const task = this.queue.then(async () => {
      const store = await this.readStore()
      const { result, changed } = await action(store)
      if (changed) await this.writeStore(store)
      return result
    })
    this.queue = task.catch(() => {})
    return task
  }

  async readStore() {
    let data
    try {
      data = await fs.readFile(this.storePath, 'utf8')
    } catch (error) {
      throw error
    }
    let store
    try {
      store = JSON.parse(data)
    } catch {
      throw new Error('本地兑换码库格式不正确')
    }
    validateStore(store)
    return store
  }

  async writeStore(store) {
    store.version = STORE_VERSION
    store.updated_at = new Date().toISOString()
    await atomicWriteJSON(this.storePath, store)
  }
}

export function createLocalOfferReplyPlugin(env) {
  let service
  let servicePromise

  async function getService() {
    if (service) return service
    if (!servicePromise) {
      servicePromise = LocalOfferReplyService.create({
        storePath: (env.LIFTTAGS_OFFER_STORE_PATH || '').trim(),
      }).then((value) => {
        service = value
        return value
      }).catch((error) => {
        servicePromise = undefined
        throw error
      })
    }
    return servicePromise
  }

  function install(server) {
    server.middlewares.use(async (req, res, next) => {
      const requestURL = new URL(req.url || '/', 'http://127.0.0.1')
      if (!requestURL.pathname.startsWith('/local-offer-reply/')) {
        next()
        return
      }
      if (!isAllowedOrigin(req.headers.origin)) {
        writeJSON(res, 403, '本地兑换码服务拒绝了跨站请求')
        return
      }

      try {
        const localService = await getService()
        if (req.method === 'GET' && requestURL.pathname === '/local-offer-reply/status') {
          writeOK(res, await localService.status())
          return
        }
        if (req.method === 'GET' && requestURL.pathname === '/local-offer-reply/codes') {
          writeOK(res, await localService.list({
            search: requestURL.searchParams.get('search') || '',
            status: requestURL.searchParams.get('status') || 'all',
            page: requestURL.searchParams.get('page') || 1,
            pageSize: requestURL.searchParams.get('page_size') || 50,
          }))
          return
        }
        if (req.method === 'POST' && requestURL.pathname === '/local-offer-reply/import') {
          if (!String(req.headers['content-type'] || '').toLowerCase().startsWith('text/csv')) {
            writeJSON(res, 415, '仅支持 CSV 文件')
            return
          }
          const body = await readBody(req, MAX_CSV_BODY_BYTES)
          const encodedName = String(req.headers['x-offer-filename'] || 'uploaded.csv')
          let fileName = 'uploaded.csv'
          try {
            fileName = decodeURIComponent(encodedName)
          } catch {}
          writeOK(res, await localService.importCSV(body, fileName))
          return
        }
        if (req.method === 'POST' && requestURL.pathname === '/local-offer-reply/replies') {
          const payload = await readJSONBody(req)
          const input = typeof payload?.input === 'string' ? payload.input.trim() : ''
          if (!input || input.length > 256) {
            writeJSON(res, 400, '兑换序号 ID 或兑换码无效')
            return
          }
          writeOK(res, await localService.generate(input))
          return
        }
        if (req.method === 'POST' && requestURL.pathname === '/local-offer-reply/usage') {
          const payload = await readJSONBody(req)
          if (typeof payload?.range !== 'string' || typeof payload?.used !== 'boolean') {
            writeJSON(res, 400, '兑换码状态参数无效')
            return
          }
          writeOK(res, await localService.setUsage(payload.range, payload.used))
          return
        }
        writeJSON(res, 404, '本地兑换码接口不存在')
      } catch (error) {
        if (error?.code === 'BODY_TOO_LARGE') {
          writeJSON(res, 413, '本地文件或请求体过大')
        } else if (error instanceof LocalOfferReplyInputError) {
          writeJSON(res, 400, error.message)
        } else {
          writeJSON(res, 503, error?.message || '本地兑换码服务不可用')
        }
      }
    })
  }

  return {
    name: 'lifttags-local-offer-reply',
    configureServer: install,
    configurePreviewServer: install,
  }
}

function createEmptyStore() {
  const now = new Date().toISOString()
  return { version: STORE_VERSION, created_at: now, updated_at: now, codes: [], imports: [] }
}

function createStoredCode(id, record, sourceFile, importedAt, usedSource) {
  return {
    id,
    code: record.code,
    url: record.url,
    used: Boolean(usedSource),
    used_at: null,
    used_source: usedSource,
    source_file: sourceFile,
    imported_at: importedAt,
    updated_at: importedAt,
  }
}

function validateStore(store) {
  if (!store || store.version !== STORE_VERSION || !Array.isArray(store.codes) || !Array.isArray(store.imports)) {
    throw new Error('本地兑换码库版本或结构无效')
  }
  const ids = new Set()
  const codes = new Set()
  for (const item of store.codes) {
    if (!Number.isSafeInteger(item.id) || item.id < 1 || typeof item.code !== 'string' || typeof item.url !== 'string' || typeof item.used !== 'boolean') {
      throw new Error('本地兑换码库包含无效记录')
    }
    item.code = item.code.trim().toUpperCase()
    item.url = item.url.trim()
    if (!item.code || !item.url || ids.has(item.id) || codes.has(item.code)) {
      throw new Error('本地兑换码库包含重复或空记录')
    }
    ids.add(item.id)
    codes.add(item.code)
  }
  store.codes.sort((left, right) => left.id - right.id)
}

function parseOfferCSV(data) {
  let records
  try {
    records = parse(data, { bom: true, skip_empty_lines: true, trim: true })
  } catch (error) {
    throw new LocalOfferReplyInputError(`CSV 解析失败：${error.message}`)
  }
  if (!records.length) throw new LocalOfferReplyInputError('兑换码 CSV 为空')
  const seen = new Map()
  return records.map((record, index) => {
    if (!Array.isArray(record) || record.length !== 2) {
      throw new LocalOfferReplyInputError(`兑换码 CSV 第 ${index + 1} 行必须只有兑换码和链接两列`)
    }
    const code = String(record[0] || '').trim().toUpperCase()
    const url = String(record[1] || '').trim()
    if (!code || !url) throw new LocalOfferReplyInputError(`兑换码 CSV 第 ${index + 1} 行缺少兑换码或链接`)
    if (seen.has(code)) {
      throw new LocalOfferReplyInputError(`兑换码 CSV 第 ${index + 1} 行与第 ${seen.get(code)} 行兑换码重复`)
    }
    seen.set(code, index + 1)
    return { code, url }
  })
}

function findStoredCode(input, codes) {
  if (!input) throw new LocalOfferReplyInputError('兑换序号 ID 或兑换码不能为空')
  let item
  if (/^\d+$/.test(input)) {
    const id = Number(input)
    item = codes.find((candidate) => candidate.id === id)
    if (!item) throw new LocalOfferReplyInputError(`兑换序号 ${input} 不存在`)
  } else {
    const normalizedCode = input.toUpperCase()
    item = codes.find((candidate) => candidate.code === normalizedCode)
    if (!item) throw new LocalOfferReplyInputError(`兑换码 "${input}" 不在本地库中`)
  }
  return item
}

function parseSequenceRange(value) {
  const text = String(value || '').trim()
  if (!text) throw new LocalOfferReplyInputError('请输入兑换序号范围')
  const ids = new Set()
  const parts = text.split(/[,，\s]+/).filter(Boolean)
  for (const part of parts) {
    const match = part.match(/^(\d+)(?:\s*[-~至]\s*(\d+))?$/)
    if (!match) throw new LocalOfferReplyInputError(`序号范围 "${part}" 格式不正确`)
    const start = Number(match[1])
    const end = Number(match[2] || match[1])
    if (!Number.isSafeInteger(start) || !Number.isSafeInteger(end) || start < 1 || end < start) {
      throw new LocalOfferReplyInputError(`序号范围 "${part}" 无效`)
    }
    if (end - start > 100000) throw new LocalOfferReplyInputError('单次设置的序号范围过大')
    for (let id = start; id <= end; id += 1) ids.add(id)
  }
  return [...ids].sort((left, right) => left - right)
}

function markCodeUsage(item, used, source) {
  const now = new Date().toISOString()
  item.used = used
  item.used_at = used ? now : null
  item.used_source = used ? source : null
  item.updated_at = now
}

function statusFromStore(store) {
  const usedCodes = store.codes.filter((item) => item.used)
  const nextCode = store.codes.find((item) => !item.used)
  return {
    total_count: store.codes.length,
    used_count: usedCodes.length,
    unused_count: store.codes.length - usedCodes.length,
    max_used_id: usedCodes.reduce((maximum, item) => Math.max(maximum, item.id), 0),
    next_id: nextCode?.id || 0,
    updated_at: store.updated_at,
  }
}

async function atomicWriteJSON(filePath, value) {
  const directory = path.dirname(filePath)
  await fs.mkdir(directory, { recursive: true })
  const temporaryPath = path.join(directory, `.offer-codes-${crypto.randomBytes(10).toString('hex')}.tmp`)
  let file
  try {
    file = await fs.open(temporaryPath, 'wx', 0o600)
    await file.writeFile(`${JSON.stringify(value, null, 2)}\n`, 'utf8')
    await file.sync()
    await file.close()
    file = undefined
    await fs.rename(temporaryPath, filePath)
  } finally {
    await file?.close().catch(() => {})
    await fs.rm(temporaryPath, { force: true }).catch(() => {})
  }
}

async function readJSONBody(req) {
  if (!String(req.headers['content-type'] || '').toLowerCase().startsWith('application/json')) {
    throw new LocalOfferReplyInputError('仅支持 application/json 请求')
  }
  const body = await readBody(req, MAX_JSON_BODY_BYTES)
  try {
    return JSON.parse(body.toString('utf8'))
  } catch {
    throw new LocalOfferReplyInputError('请求格式不正确')
  }
}

async function readBody(req, limit) {
  const chunks = []
  let size = 0
  for await (const chunk of req) {
    size += chunk.length
    if (size > limit) {
      const error = new Error('请求体过大')
      error.code = 'BODY_TOO_LARGE'
      throw error
    }
    chunks.push(chunk)
  }
  return Buffer.concat(chunks)
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

function writeOK(res, data) {
  res.statusCode = 200
  res.setHeader('Content-Type', 'application/json; charset=utf-8')
  res.setHeader('Cache-Control', 'no-store')
  res.end(JSON.stringify({ code: 0, message: 'ok', data }))
}

function writeJSON(res, status, message) {
  res.statusCode = status
  res.setHeader('Content-Type', 'application/json; charset=utf-8')
  res.setHeader('Cache-Control', 'no-store')
  res.end(JSON.stringify({ code: status, message }))
}

function buildReply(offer) {
  return `Hey, thanks so much for joining the lifttags giveaway! 🙌

Here’s your lifetime pro membership code: ${offer.code}

You can redeem it in either of these two ways:

Option 1 – Click the link (easiest):
${offer.url}

Just open it on your iPhone and it will apply lifetime pro automatically.

Option 2 – Enter manually in the App Store:

1. Open the App Store on your iPhone
2. Tap your profile picture (top right corner)
3. Tap “Redeem Gift Card or Code”
4. Tap “You can also enter your code manually”
5. Enter ${offer.code} and tap Redeem

Quick tip: if you ever see a paywall inside the app, simply tap “Restore Purchases” – your lifetime pro access will show up right away.

That’s it – enjoy tracking all your workouts in one place! 🏋️

We’re just a small team of two, and we’d really appreciate it if you could leave a rating or a short review on the App Store – it helps others discover lifttags too. 🫶

If you have any feedback or feature requests, feel free to reply anytime!`
}
