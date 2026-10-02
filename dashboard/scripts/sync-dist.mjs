import { cpSync, mkdirSync, readdirSync, rmSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const source = fileURLToPath(new URL('../dist/', import.meta.url))
const target = fileURLToPath(new URL('../../backend/embed/dist/', import.meta.url))
// Validate the build output before replacing the generated embedded assets.
if (!readdirSync(source).includes('index.html')) throw new Error('Missing dashboard/dist/index.html')
rmSync(target, { recursive: true, force: true })
mkdirSync(target, { recursive: true })
cpSync(source, target, { recursive: true })
console.log('Dashboard copied to backend/embed/dist')
