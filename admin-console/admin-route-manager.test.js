import assert from 'node:assert/strict'
import test from 'node:test'
import { AdminRouteManager, configuredAdminRoutes } from './admin-route-manager.js'

function makeRoutes() {
  return configuredAdminRoutes({
    ADMIN_SERVER_URL_LINE1: 'https://line1.example.com',
    ADMIN_SERVER_URL_LINE2: 'https://line2.example.com',
  })
}

test('uses the fastest reachable admin route on its first request', async () => {
  let now = 0
  const manager = new AdminRouteManager(makeRoutes(), {
    now: () => now,
    fetchImpl: async (url) => {
      now += url.hostname.startsWith('line1') ? 40 : 10
      return { ok: true }
    },
  })

  const route = await manager.routeForRequest()
  assert.equal(route.id, 'line2')
  assert.equal(manager.status().mode, 'automatic')
})

test('keeps a working route and fails over after that route fails', async () => {
  let now = 1
  const manager = new AdminRouteManager(makeRoutes(), {
    now: () => now,
    fetchImpl: async () => ({ ok: true }),
  })

  await manager.routeForRequest()
  manager.markSuccess('line2')
  manager.markFailure('line2')
  assert.equal(manager.status().current_route, 'line1')

  now += 1
  assert.equal((await manager.routeForRequest()).id, 'line1')
})

test('manual route mode pins requests to the selected line and disables failover', async () => {
  const manager = new AdminRouteManager(makeRoutes(), {
    fetchImpl: async () => ({ ok: true }),
  })

  await manager.setMode('line2')
  manager.markFailure('line2')
  assert.equal(manager.status().mode, 'line2')
  assert.equal((await manager.routeForRequest()).id, 'line2')

  await manager.setMode('automatic')
  assert.equal(manager.status().mode, 'automatic')
})
