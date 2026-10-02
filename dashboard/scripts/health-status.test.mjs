import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import ts from 'typescript'

const source = readFileSync(new URL('../src/utils/healthStatus.ts', import.meta.url), 'utf8')
const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText
const { healthBadge, isHealthResultCurrent } = await import('data:text/javascript;base64,' + Buffer.from(js).toString('base64'))
assert.equal(healthBadge({}).label, '未检测')
assert.equal(healthBadge({ status: 'normal', failCount: 0 }).label, '未检测')
assert.equal(healthBadge({ available: false }).label, '本次检测失败')
assert.equal(healthBadge({ status: 'error', failCount: 0, available: true }).label, '暂停分发')
assert.equal(healthBadge({ enabled: false, available: false }).label, '已停用')
assert.equal(healthBadge({ available: true, failCount: 5 }).label, '检测通过')
assert.equal(healthBadge({ failCount: 2 }).label, '最近请求失败')
assert.equal(isHealthResultCurrent(undefined, '2026-10-01T12:00:00Z'), false)
assert.equal(isHealthResultCurrent('2026-10-01T12:00:00Z', '2026-10-01T12:01:00Z'), false)
assert.equal(isHealthResultCurrent('2026-10-01T12:02:00Z', '2026-10-01 12:01:00'), true)
console.log('Health badge and stale-result regression checks passed')

// Render the actual Vue component too: optional Boolean props otherwise default to false.
const { parse, compileScript } = await import('@vue/compiler-sfc')
const { createSSRApp, h } = await import('vue')
const { renderToString } = await import('@vue/server-renderer')
const componentSource = readFileSync(new URL('../src/components/HealthStatusBadge.vue', import.meta.url), 'utf8')
const { descriptor } = parse(componentSource)
const compiled = compileScript(descriptor, { id: 'health-test', inlineTemplate: true })
let componentJS = ts.transpileModule(compiled.content, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText
const helperURL = 'data:text/javascript;base64,' + Buffer.from(js).toString('base64')
componentJS = componentJS.replaceAll('"vue"', JSON.stringify(import.meta.resolve('vue'))).replaceAll("'vue'", JSON.stringify(import.meta.resolve('vue'))).replaceAll("'../utils/healthStatus'", JSON.stringify(helperURL))
const { default: Badge } = await import('data:text/javascript;base64,' + Buffer.from(componentJS).toString('base64'))
for (const [props, label] of [[{}, '未检测'], [{ available: true }, '检测通过'], [{ available: false }, '本次检测失败'], [{ enabled: false, available: true }, '已停用']]) {
  const html = await renderToString(createSSRApp({ render: () => h(Badge, props) }))
  assert.ok(html.includes(label), html)
}
console.log('Vue component Boolean-prop regression checks passed')
