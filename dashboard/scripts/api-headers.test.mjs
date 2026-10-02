import assert from 'node:assert/strict'

const values = new Map()
globalThis.localStorage = {
  getItem: (key) => values.get(key) ?? null,
  setItem: (key, value) => values.set(key, value),
  removeItem: (key) => values.delete(key),
}
globalThis.window = { location: { pathname: '/', href: '/' } }

const { setAuthToken, useApi } = await import('../src/composables/useApi.ts')
setAuthToken('test-bearer-token')

let lastRequest
globalThis.fetch = async (_url, options) => {
  lastRequest = options
  return new Response('{"ok":true}', {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

const api = useApi()
await api.importCustomData(new FormData())
const multipartHeaders = new Headers(lastRequest.headers)
assert.equal(multipartHeaders.get('Authorization'), 'Bearer test-bearer-token')
assert.equal(multipartHeaders.has('Content-Type'), false)

await api.importCustomData({ version: 'test' })
const jsonHeaders = new Headers(lastRequest.headers)
assert.equal(jsonHeaders.get('Authorization'), 'Bearer test-bearer-token')
assert.equal(jsonHeaders.get('Content-Type'), 'application/json')
assert.equal(lastRequest.body, '{"version":"test"}')

console.log('API request header checks passed')

for (const call of [api.getTags, api.getHealthStatus, api.checkAuth]) {
  await call()
  assert.equal(new Headers(lastRequest.headers).get('Authorization'), 'Bearer test-bearer-token')
}
let clicked = false
const link = { click() { clicked = true } }
globalThis.document = { createElement: () => link }
await api.exportCustomData(['config'])
assert.equal(clicked, true)
assert.equal(link.download, 'pichub-backup.json')
assert.equal(new Headers(lastRequest.headers).get('Authorization'), 'Bearer test-bearer-token')
console.log('Authenticated reads and backup download checks passed')
