<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useLocalStorage } from '@vueuse/core'
import { Sparkles, User, Key, LogIn, ShieldCheck, ArrowRight } from 'lucide-vue-next'
import { useApi, getAuthToken, setAuthToken } from '../composables/useApi'
import type { Settings } from '../types'

const router = useRouter()
const { login, getSettings, updateSettings } = useApi()

const username = ref('')
const password = ref('')
const loading = ref(false)
const errorMsg = ref('')
const loginNotEnabled = ref(false)

// 首次运行设置模式: 登录已启用但未配置账号密码
const setupMode = ref(false)
const setupUsername = ref('')
const setupPassword = ref('')
const setupConfirm = ref('')

const remember = useLocalStorage('pichub_remember_login', { remember: false, username: '', password: '' })

onMounted(async () => {
  // 已登录直接进控制台
  if (getAuthToken()) {
    router.replace('/')
    return
  }
  try {
    const s = await getSettings()
    loginNotEnabled.value = !s.login_enabled
    setupMode.value = !!s.login_enabled && !s.admin_username
  } catch {
    // 后端不可达时仍展示登录表单
  }
  if (remember.value.remember) {
    if (remember.value.username) username.value = remember.value.username
    if (remember.value.password) password.value = remember.value.password
  }
})

async function doLogin() {
  if (!username.value.trim() || !password.value) {
    errorMsg.value = '请输入用户名与密码'
    return
  }
  errorMsg.value = ''
  loading.value = true
  try {
    await login(username.value.trim(), password.value)
    // 记住密码: 勾选则保存用户名+密码, 否则清空
    if (remember.value.remember) {
      remember.value.username = username.value.trim()
      remember.value.password = password.value
    } else {
      remember.value.username = ''
      remember.value.password = ''
    }
    password.value = ''
    router.replace('/')
  } catch (e: any) {
    errorMsg.value = e.message || '登录失败'
  } finally {
    loading.value = false
  }
}

const setupError = computed(() => errorMsg.value)

async function doSetup() {
  errorMsg.value = ''
  if (!setupUsername.value.trim()) {
    errorMsg.value = '请设置管理员用户名'
    return
  }
  if (setupPassword.value.length < 6) {
    errorMsg.value = '密码至少 6 位'
    return
  }
  if (setupPassword.value !== setupConfirm.value) {
    errorMsg.value = '两次输入的密码不一致'
    return
  }
  loading.value = true
  try {
    const s = await getSettings()
    const payload: Settings = { ...s, login_enabled: true, admin_username: setupUsername.value.trim(), admin_password: setupPassword.value }
    await updateSettings(payload)
    // 设置成功自动登录进入控制台
    await login(setupUsername.value.trim(), setupPassword.value)
    setupPassword.value = ''
    setupConfirm.value = ''
    router.replace('/')
  } catch (e: any) {
    errorMsg.value = e.message || '设置失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen bg-morandi-bg flex flex-col items-center justify-center px-4 font-sans">
    <div class="w-full max-w-sm">
      <!-- Brand -->
      <div class="flex flex-col items-center mb-8">
        <div class="w-14 h-14 rounded-2xl bg-morandi-sage text-white flex items-center justify-center shadow-lg shadow-morandi-sage/20 mb-4">
          <Sparkles class="w-7 h-7" />
        </div>
        <h1 class="text-xl font-bold text-morandi-text tracking-tight">PicHub</h1>
        <p class="text-xs text-morandi-muted mt-1">图源聚合中转引擎 · 管理控制台</p>
      </div>

      <!-- Card -->
      <div class="bg-morandi-card rounded-2xl border border-morandi-borderSoft shadow-morandi p-6">
        <!-- 未启用登录 -->
        <div v-if="loginNotEnabled" class="text-center py-6">
          <p class="text-sm text-morandi-muted mb-4">当前未启用登录保护，可直接访问控制台</p>
          <button
            class="px-4 py-2 bg-morandi-sage text-white rounded-xl text-xs font-medium hover:bg-morandi-sage-dark transition-colors"
            @click="router.replace('/')"
          >
            进入控制台
          </button>
        </div>

        <!-- 首次运行: 设置管理员账号密码 -->
        <div v-else-if="setupMode">
          <div class="flex items-center gap-2.5 mb-5">
            <div class="w-8 h-8 rounded-xl bg-morandi-lavender/20 text-morandi-lavender-dark flex items-center justify-center shrink-0">
              <ShieldCheck class="w-4 h-4" />
            </div>
            <div>
              <h2 class="text-sm font-bold text-morandi-text">首次运行设置</h2>
              <p class="text-[11px] text-morandi-muted">请设置管理员账号与密码，登录保护将立即启用</p>
            </div>
          </div>

          <form @submit.prevent="doSetup" class="space-y-4">
            <div v-if="setupError" class="px-3 py-2 bg-rose-50 border border-rose-200 text-rose-600 text-xs rounded-xl">
              {{ setupError }}
            </div>
            <div>
              <label class="block text-xs font-medium text-morandi-text mb-1.5">管理员用户名</label>
              <div class="relative">
                <User class="w-4 h-4 text-morandi-light absolute left-3 top-1/2 -translate-y-1/2" />
                <input
                  v-model.trim="setupUsername"
                  type="text"
                  autocomplete="username"
                  placeholder="admin"
                  class="w-full pl-9 pr-3 py-2.5 bg-morandi-bg border border-morandi-border rounded-xl text-sm text-morandi-text placeholder:text-morandi-light focus:outline-none focus:border-morandi-sage focus:ring-2 focus:ring-morandi-sage/20 transition-all"
                />
              </div>
            </div>
            <div>
              <label class="block text-xs font-medium text-morandi-text mb-1.5">密码（至少 6 位）</label>
              <div class="relative">
                <Key class="w-4 h-4 text-morandi-light absolute left-3 top-1/2 -translate-y-1/2" />
                <input
                  v-model="setupPassword"
                  type="password"
                  autocomplete="new-password"
                  placeholder="请输入密码"
                  class="w-full pl-9 pr-3 py-2.5 bg-morandi-bg border border-morandi-border rounded-xl text-sm text-morandi-text placeholder:text-morandi-light focus:outline-none focus:border-morandi-sage focus:ring-2 focus:ring-morandi-sage/20 transition-all"
                />
              </div>
            </div>
            <div>
              <label class="block text-xs font-medium text-morandi-text mb-1.5">确认密码</label>
              <div class="relative">
                <Key class="w-4 h-4 text-morandi-light absolute left-3 top-1/2 -translate-y-1/2" />
                <input
                  v-model="setupConfirm"
                  type="password"
                  autocomplete="new-password"
                  placeholder="再次输入密码"
                  class="w-full pl-9 pr-3 py-2.5 bg-morandi-bg border border-morandi-border rounded-xl text-sm text-morandi-text placeholder:text-morandi-light focus:outline-none focus:border-morandi-sage focus:ring-2 focus:ring-morandi-sage/20 transition-all"
                />
              </div>
            </div>

            <button
              type="button"
              :disabled="loading"
              @click="doSetup"
              class="w-full flex items-center justify-center gap-1.5 px-4 py-2.5 bg-morandi-sage text-white rounded-xl text-xs font-medium hover:bg-morandi-sage-dark disabled:opacity-60 transition-colors"
            >
              <ArrowRight class="w-3.5 h-3.5" />
              {{ loading ? '设置中…' : '创建账号并进入控制台' }}
            </button>
          </form>
        </div>

        <!-- 正常登录 -->
        <form v-else @submit.prevent="doLogin" class="space-y-4">
          <div v-if="errorMsg" class="px-3 py-2 bg-rose-50 border border-rose-200 text-rose-600 text-xs rounded-xl">
            {{ errorMsg }}
          </div>
          <div>
            <label class="block text-xs font-medium text-morandi-text mb-1.5">用户名</label>
            <div class="relative">
              <User class="w-4 h-4 text-morandi-light absolute left-3 top-1/2 -translate-y-1/2" />
              <input
                v-model.trim="username"
                type="text"
                autocomplete="username"
                placeholder="请输入用户名"
                class="w-full pl-9 pr-3 py-2.5 bg-morandi-bg border border-morandi-border rounded-xl text-sm text-morandi-text placeholder:text-morandi-light focus:outline-none focus:border-morandi-sage focus:ring-2 focus:ring-morandi-sage/20 transition-all"
              />
            </div>
          </div>
          <div>
            <label class="block text-xs font-medium text-morandi-text mb-1.5">密码</label>
            <div class="relative">
              <Key class="w-4 h-4 text-morandi-light absolute left-3 top-1/2 -translate-y-1/2" />
              <input
                v-model="password"
                type="password"
                autocomplete="current-password"
                placeholder="请输入密码"
                class="w-full pl-9 pr-3 py-2.5 bg-morandi-bg border border-morandi-border rounded-xl text-sm text-morandi-text placeholder:text-morandi-light focus:outline-none focus:border-morandi-sage focus:ring-2 focus:ring-morandi-sage/20 transition-all"
              />
            </div>
          </div>

          <div class="flex items-center justify-between">
            <label class="flex items-center gap-2 text-xs text-morandi-muted cursor-pointer select-none">
              <input v-model="remember.remember" type="checkbox" class="w-3.5 h-3.5 rounded border-morandi-border accent-morandi-sage focus:ring-morandi-sage/20" />
              记住密码
            </label>
            <button
              type="button"
              :disabled="loading"
              @click="doLogin"
              class="flex items-center gap-1.5 px-4 py-2 bg-morandi-sage text-white rounded-xl text-xs font-medium hover:bg-morandi-sage-dark disabled:opacity-60 transition-colors"
            >
              <LogIn class="w-3.5 h-3.5" />
              {{ loading ? '登录中…' : '登录' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
