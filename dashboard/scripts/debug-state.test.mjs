import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import ts from 'typescript'
const keepAlive=setInterval(()=>{},1000)
const originalTimeout=AbortSignal.timeout
try {
  AbortSignal.timeout=ms=>{assert.equal(ms,8000);return originalTimeout(20)}
  globalThis.debugApi={getSourceDebug:async()=>undefined,enableSourceDebug:async id=>({enabled:true,source_id:id,source_name:'test'}),disableSourceDebug:async()=>({enabled:false})}
  let source=readFileSync(new URL('../src/composables/useSourceDebug.ts',import.meta.url),'utf8').replace("import { useApi } from './useApi'",'const useApi=()=>globalThis.debugApi')
  let js=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText.replace("'vue'",JSON.stringify(import.meta.resolve('vue')))
  const {useSourceDebug}=await import('data:text/javascript;base64,'+Buffer.from(js).toString('base64'))
  const store=useSourceDebug()
  await store.refresh()
  assert.equal(store.loading.value,false)
  assert.match(store.error.value,/无效/)
  debugApi.getSourceDebug=signal=>new Promise((resolve,reject)=>signal.addEventListener('abort',()=>reject(signal.reason),{once:true}))
  await store.refresh()
  assert.equal(store.loading.value,false)
  assert.match(store.error.value,/超时/)
  let resolveRead;let calls=0
  debugApi.getSourceDebug=()=>{calls++;return new Promise(resolve=>{resolveRead=resolve})}
  const read1=store.refresh(),read2=store.refresh()
  assert.equal(calls,1)
  await store.enable(41)
  resolveRead({enabled:false})
  await Promise.all([read1,read2])
  assert.equal(store.state.value.source_id,41)
  assert.equal(useSourceDebug().state.value.enabled,true)
  await store.disable()
  assert.equal(store.state.value.enabled,false)
  console.log('PASS: invalid response/timeout clears spinner; coalesced reads; stale reads cannot overwrite shared mode')
} finally {AbortSignal.timeout=originalTimeout;clearInterval(keepAlive)}
