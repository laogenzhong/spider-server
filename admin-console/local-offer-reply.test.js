import assert from 'node:assert/strict'
import { promises as fs } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { LocalOfferReplyInputError, LocalOfferReplyService } from './local-offer-reply.js'

const initialCSV = 'FIRSTCODE123456789,"https://example.com/redeem?from=a,b"\nSECONDCODE12345678,https://example.com/second\nTHIRDCODE123456789,https://example.com/third\n'

async function makeService() {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'lifttags-offer-library-'))
  const storePath = path.join(directory, 'admin-store.json')
  const service = await LocalOfferReplyService.create({ storePath })
  await service.importCSV(Buffer.from(initialCSV), 'initial.csv')
  await service.setUsage('1-2', true)
  return { directory, service, storePath }
}

test('creates an independent local library and persists imported usage state', async (t) => {
  const { directory, service, storePath } = await makeService()
  t.after(() => fs.rm(directory, { recursive: true, force: true }))

  assert.deepEqual(await service.status(), {
    total_count: 3,
    used_count: 2,
    unused_count: 1,
    max_used_id: 2,
    next_id: 3,
    updated_at: (await service.status()).updated_at,
  })

  const used = await service.list({ status: 'used' })
  assert.deepEqual(used.items.map((item) => item.id), [1, 2])
  assert.equal(used.items[0].used_source, 'manual_batch')

  const store = JSON.parse(await fs.readFile(storePath, 'utf8'))
  assert.equal(store.version, 1)
  assert.equal(store.codes.length, 3)
  assert.equal(store.imports[0].migration, false)
})

test('merges uploaded CSV files without changing existing ids or usage', async (t) => {
  const { directory, service } = await makeService()
  t.after(() => fs.rm(directory, { recursive: true, force: true }))

  const result = await service.importCSV(
    Buffer.from('SECONDCODE12345678,https://example.com/second-updated\nFOURTHCODE12345678,https://example.com/fourth\n'),
    'new-codes.csv',
  )
  assert.equal(result.added_count, 1)
  assert.equal(result.updated_count, 1)
  assert.equal(result.existing_count, 1)

  const all = await service.list({ pageSize: 100 })
  assert.deepEqual(all.items.map((item) => item.id), [1, 2, 3, 4])
  assert.equal(all.items[1].used, true)
  assert.equal(all.items[1].url, 'https://example.com/second-updated')
  assert.equal(all.items[3].used, false)
})

test('generating replies and batch ranges update local usage state', async (t) => {
  const { directory, service } = await makeService()
  t.after(() => fs.rm(directory, { recursive: true, force: true }))

  const generated = await service.generate('3')
  assert.equal(generated.id, 3)
  assert.equal(generated.already_used, false)
  assert.match(generated.reply, /THIRDCODE123456789/)

  const duplicate = await service.generate('thirdcode123456789')
  assert.equal(duplicate.already_used, true)

  const restored = await service.setUsage('1-2', false)
  assert.equal(restored.changed_count, 2)
  const marked = await service.setUsage('1-2, 3', true)
  assert.equal(marked.selected_count, 3)
  assert.equal(marked.changed_count, 2)
  assert.equal((await service.status()).used_count, 3)
})

test('rejects invalid or missing sequence ranges', async (t) => {
  const { directory, service } = await makeService()
  t.after(() => fs.rm(directory, { recursive: true, force: true }))

  await assert.rejects(
    () => service.setUsage('1-99', true),
    (error) => error instanceof LocalOfferReplyInputError && /序号 4 不存在/.test(error.message),
  )
  await assert.rejects(
    () => service.setUsage('9-2', true),
    (error) => error instanceof LocalOfferReplyInputError && /无效/.test(error.message),
  )
})
