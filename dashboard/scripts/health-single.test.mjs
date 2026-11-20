import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import ts from 'typescript'

const calls = []
let resolveCheck, rejectCheck
let batchCalls = 0
const a = { id: 1, name: 'A', available: false, latency_ms: 0, checked_at: '2026-11-20T01:00:00Z' }
const b = { id: 2, name: 'B', available: true, latency_ms: 10, checked_at: '2026-11-20T01:00:00Z' }
globalThis.__healthTestApi = {
  getHealthStatus: async () => ({ results: [a, b], last_run: a.checked_at }),
  healthCheck: async () => { batchCalls++; return [a, b] },
  healthCheckSource: id => {
    calls.push(id)
    return new Promise((resolve, reject) => { resolveCheck = resolve; rejectCheck = reject })
  },
}
try {
  const mock = 'data:text/javascript;base64,' + Buffer.from('export const useApi = () => globalThis.__healthTestApi').toString('base64')
  const source = readFileSync(new URL('../src/composables/useHealthCheck.ts', import.meta.url), 'utf8')
  let js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText
  js = js.replaceAll("'vue'", JSON.stringify(import.meta.resolve('vue'))).replaceAll("'./useApi'", JSON.stringify(mock))
  const { useHealthCheck } = await import('data:text/javascript;base64,' + Buffer.from(js).toString('base64'))
  const view = useHealthCheck()
  await view.loadCached()
  const beforeB = view.results.value[1]
  const pending = view.runSourceCheck(1)
  assert.equal(view.checkingIds.value.has(1), true)
  assert.equal(view.checkingIds.value.has(2), false)
  await view.runSourceCheck(1)
  await view.runCheck()
  assert.deepEqual(calls, [1], 'repeated clicks must not duplicate requests')
  assert.equal(batchCalls, 0, 'individual checks must not trigger a batch')
  const updated = { ...a, available: true, latency_ms: 25, checked_at: '2026-11-20T02:00:00Z' }
  resolveCheck(updated)
  await pending
  assert.equal(view.results.value[0].available, true)
  assert.equal(view.results.value[1], beforeB, 'other rows must stay unchanged')
  assert.equal(view.summary.value.available, 2)
  assert.equal(view.lastRun.value, updated.checked_at)
  assert.equal(view.checkingIds.value.size, 0)
  const failed = view.runSourceCheck(1)
  rejectCheck(new Error('连接超时'))
  await failed
  assert.equal(view.rowErrors.value[1], '连接超时')
  assert.equal(view.error.value, '')
  assert.equal(view.results.value[0].available, true, 'request error must retain prior result')
  assert.equal(view.checkingIds.value.size, 0)
  const retry = view.runSourceCheck(1)
  assert.equal(view.rowErrors.value[1], undefined)
  resolveCheck(updated)
  await retry
  console.log('Single-source health checks: isolation, duplicate guard, pending state and retry passed')
} finally {
  delete globalThis.__healthTestApi
}
