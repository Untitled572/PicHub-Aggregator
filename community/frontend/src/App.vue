<script setup lang="ts">
import { RouterView, RouterLink, useRoute } from 'vue-router'
import { Compass, PlusCircle, Sparkles, ExternalLink, ShieldCheck } from 'lucide-vue-next'

const route = useRoute()
function isActive(path: string) {
  return route.path === path
}
</script>

<template>
  <div class="min-h-screen bg-morandi-bg text-morandi-text font-sans">
    <!-- Top Header -->
    <header class="bg-white/80 backdrop-blur-md border-b border-morandi-border/60 sticky top-0 z-30 shadow-xs">
      <div class="max-w-6xl mx-auto px-4 h-16 flex items-center justify-between">
        <!-- Logo -->
        <RouterLink to="/" class="flex items-center gap-2.5 group">
          <div class="w-9 h-9 rounded-xl bg-morandi-ocean text-white flex items-center justify-center shadow-md shadow-morandi-ocean/20 transition-transform group-hover:scale-105">
            <Sparkles class="w-4 h-4" />
          </div>
          <div>
            <div class="flex items-center gap-1.5">
              <span class="font-bold text-base tracking-tight text-morandi-text">PicHub Community</span>
              <span class="text-[10px] px-1.5 py-0.2 bg-morandi-ocean/15 text-morandi-ocean-dark font-medium rounded-md">Serverless</span>
            </div>
            <p class="text-[10px] text-morandi-muted leading-tight">开源 API 解析规则共享社区</p>
          </div>
        </RouterLink>

        <!-- Nav Links -->
        <div class="flex items-center gap-2">
          <RouterLink
            to="/"
            class="flex items-center gap-1.5 px-3.5 py-2 text-xs font-medium rounded-xl transition-all"
            :class="isActive('/')
              ? 'bg-morandi-ocean-light text-morandi-ocean-dark font-semibold border border-morandi-ocean/20'
              : 'text-morandi-muted hover:bg-morandi-hover hover:text-morandi-text'"
          >
            <Compass class="w-4 h-4" />
            <span>规则广场</span>
            <span class="text-[10px] px-1 py-0.2 bg-amber-100 text-amber-800 font-bold rounded">待开发</span>
          </RouterLink>

          <RouterLink
            to="/submit"
            class="flex items-center gap-1.5 px-3.5 py-2 text-xs font-medium rounded-xl transition-all"
            :class="isActive('/submit')
              ? 'bg-morandi-ocean text-white font-semibold shadow-sm'
              : 'bg-morandi-ocean-light text-morandi-ocean-dark hover:bg-morandi-ocean/20'"
          >
            <PlusCircle class="w-4 h-4" />
            <span>提交规则</span>
          </RouterLink>


          <div class="h-4 w-px bg-morandi-border/60 mx-1"></div>

          <a
            href="/"
            target="_blank"
            class="flex items-center gap-1 px-3 py-2 text-xs font-medium text-morandi-muted hover:text-morandi-sage-dark hover:bg-morandi-sage-light/60 rounded-xl transition-colors"
            title="返回 Docker 节点控制台"
          >
            <ShieldCheck class="w-3.5 h-3.5 text-morandi-sage" />
            <span>节点控制台</span>
            <ExternalLink class="w-3 h-3 text-morandi-light" />
          </a>
        </div>
      </div>
    </header>

    <!-- Main Content Container -->
    <main class="max-w-6xl mx-auto px-4 py-8">
      <RouterView />
    </main>
  </div>
</template>

