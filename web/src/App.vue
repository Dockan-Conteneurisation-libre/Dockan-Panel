<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import Icon from './components/Icon.vue'
import LangSelect from './components/LangSelect.vue'
import LoginView from './views/LoginView.vue'
import { nav } from './router'
import { t, type Key } from './i18n'
import { api } from './lib/api'
import { state, loadSession, toast } from './lib/state'
import { dialog, closeDialog } from './lib/confirm'

const route = useRoute()
const menuOpen = ref(false)
const loadError = ref('')

onMounted(async () => {
  try {
    await loadSession()
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : String(e)
  }
})
watch(() => route.fullPath, () => { menuOpen.value = false })

const groups = computed(() => {
  const out: { label: Key; items: typeof nav }[] = []
  for (const item of nav) {
    let g = out.find((x) => x.label === item.group)
    if (!g) out.push((g = { label: item.group, items: [] }))
    g.items.push(item)
  }
  return out
})

function active(path: string): boolean {
  return path === '/' ? route.path === '/' : route.path.startsWith(path)
}

async function logout(): Promise<void> {
  try {
    await api.post('/api/logout')
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
  }
  await loadSession()
}
</script>

<template>
  <div v-if="!state.ready" class="grid min-h-screen place-items-center text-muted">
    <p v-if="loadError" class="text-danger">{{ loadError }}</p>
    <p v-else>{{ t('common.loading') }}</p>
  </div>

  <LoginView v-else-if="!state.session?.user" />

  <div v-else class="shell">
    <aside class="side" :class="{ open: menuOpen }">
      <div class="brand">
        <img src="/dockan-logo.svg" alt="" width="34" height="34" />
        <div>
          <strong>{{ t('app.name') }}</strong>
          <span>{{ state.session.version }}</span>
        </div>
      </div>
      <nav :aria-label="t('nav.menu')">
        <div v-for="g in groups" :key="g.label" class="group">
          <p>{{ t(g.label) }}</p>
          <RouterLink v-for="item in g.items" :key="item.path" :to="item.path" :class="{ on: active(item.path) }">
            <Icon :name="item.icon" />
            {{ t(item.label) }}
          </RouterLink>
        </div>
      </nav>
      <div class="side-foot">
        <span class="who"><Icon name="user" :size="16" /> {{ state.session.user.username }}</span>
        <button class="btn btn-ghost btn-sm text-[#cfe3d8]" @click="logout">
          <Icon name="logout" /> {{ t('nav.logout') }}
        </button>
      </div>
    </aside>
    <div v-if="menuOpen" class="scrim" @click="menuOpen = false" />

    <div class="main">
      <header class="top">
        <button class="btn btn-ghost btn-icon menu-btn" :aria-label="t('nav.menu')" @click="menuOpen = true">
          <Icon name="menu" />
        </button>
        <div class="flex-1" />
        <span v-if="state.busy > 0" class="busy"><span class="spin" /> {{ t('activity.working') }}</span>
        <button class="btn btn-sm" :class="{ 'btn-primary': state.activityOpen }" @click="state.activityOpen = !state.activityOpen">
          <Icon name="activity" />
          <span class="hide-sm">{{ t('nav.activity') }}</span>
          <span v-if="state.activity.length" class="count">{{ state.activity.length }}</span>
        </button>
        <LangSelect />
      </header>

      <main class="content">
        <RouterView v-slot="{ Component }">
          <component :is="Component" />
        </RouterView>
      </main>
    </div>

    <aside v-if="state.activityOpen" class="activity" :aria-label="t('activity.title')">
      <div class="flex items-center justify-between gap-2 border-b border-line px-4 py-3">
        <strong class="text-[0.92rem]">{{ t('activity.title') }}</strong>
        <div class="flex gap-1">
          <button class="btn btn-ghost btn-sm" :disabled="!state.activity.length" @click="state.activity.splice(0)">{{ t('activity.clear') }}</button>
          <button class="btn btn-ghost btn-icon btn-sm" :aria-label="t('common.close')" @click="state.activityOpen = false"><Icon name="close" /></button>
        </div>
      </div>
      <div class="flex-1 overflow-y-auto p-3">
        <p v-if="!state.activity.length" class="px-2 py-6 text-center text-[0.86rem] text-muted">{{ t('activity.empty') }}</p>
        <article v-for="a in state.activity" :key="a.id" class="entry">
          <header>
            <Icon :name="a.ok ? 'check' : 'alert'" :size="15" :class="a.ok ? 'text-leaf' : 'text-danger'" />
            <strong>{{ a.title }}</strong>
            <time>{{ a.at.toLocaleTimeString() }}</time>
          </header>
          <pre class="pre">{{ a.output || '—' }}</pre>
        </article>
      </div>
    </aside>
  </div>

  <div class="toasts" aria-live="polite">
    <div v-for="x in state.toasts" :key="x.id" class="toast" :class="x.kind">
      <Icon :name="x.kind === 'ok' ? 'check' : 'alert'" :size="16" />
      <span>{{ x.text }}</span>
    </div>
  </div>

  <Teleport to="body">
    <div v-if="dialog.open" class="fixed inset-0 z-[60] grid place-items-center bg-[rgb(8_20_14/0.5)] p-4" @click.self="closeDialog(false)">
      <div class="card w-full max-w-[440px] p-5 shadow-2xl" role="alertdialog" aria-modal="true">
        <p class="mt-0 mb-5 text-[0.95rem] break-words">{{ dialog.text }}</p>
        <div class="flex justify-end gap-2">
          <button class="btn" @click="closeDialog(false)">{{ t('common.cancel') }}</button>
          <button class="btn" :class="dialog.danger ? 'btn-solid-danger' : 'btn-primary'" @click="closeDialog(true)">{{ t('common.confirm') }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.shell { display: flex; min-height: 100vh; }
.side {
  position: sticky; top: 0; height: 100vh; width: 248px; flex: none;
  display: flex; flex-direction: column; background: var(--color-pine); color: #cfe3d8;
  border-right: 1px solid var(--color-pine-line); z-index: 30;
}
.brand { display: flex; align-items: center; gap: 0.7rem; padding: 1.1rem 1.1rem 1rem; }
.brand img { border-radius: 8px; }
.brand strong { display: block; color: #fff; font-size: 0.98rem; letter-spacing: -0.01em; }
.brand span { display: block; color: var(--color-mint); font: 0.72rem/1.3 var(--font-mono); opacity: 0.85; }
nav { flex: 1; overflow-y: auto; padding: 0.25rem 0.7rem 1rem; }
.group { margin-top: 0.9rem; }
.group p { margin: 0 0 0.3rem; padding: 0 0.6rem; font-size: 0.68rem; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; color: #7fa594; }
nav a {
  display: flex; align-items: center; gap: 0.7rem; min-height: 38px; padding: 0 0.65rem; border-radius: 7px;
  color: #cfe3d8; text-decoration: none; font-size: 0.9rem; font-weight: 550;
}
nav a:hover { background: var(--color-pine-2); color: #fff; }
nav a.on { background: var(--color-pine-3); color: #fff; box-shadow: inset 3px 0 0 var(--color-mint); }
nav a.on svg { color: var(--color-mint); }
.side-foot { display: flex; align-items: center; justify-content: space-between; gap: 0.5rem; padding: 0.8rem 0.9rem; border-top: 1px solid var(--color-pine-line); }
.who { display: inline-flex; align-items: center; gap: 0.4rem; font-size: 0.84rem; color: #fff; min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.side-foot .btn:hover { background: var(--color-pine-2); border-color: var(--color-pine-line); }

.main { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.top {
  position: sticky; top: 0; z-index: 20; display: flex; align-items: center; gap: 0.6rem;
  min-height: 56px; padding: 0 1.4rem; background: color-mix(in srgb, var(--color-paper) 88%, transparent);
  backdrop-filter: blur(8px); border-bottom: 1px solid var(--color-line);
}
.menu-btn { display: none; }
.content { flex: 1; width: 100%; max-width: 1320px; margin: 0 auto; padding: 1.8rem 1.4rem 3rem; }
.busy { display: inline-flex; align-items: center; gap: 0.45rem; font-size: 0.82rem; color: var(--color-muted); }
.spin { width: 14px; height: 14px; border-radius: 50%; border: 2px solid var(--color-line); border-top-color: var(--color-teal); animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
.count { min-width: 18px; padding: 0 5px; border-radius: 9px; background: var(--color-code); color: var(--color-ink); font-size: 0.7rem; line-height: 18px; }

.activity {
  position: sticky; top: 0; height: 100vh; width: 380px; flex: none; display: flex; flex-direction: column;
  background: var(--color-surface); border-left: 1px solid var(--color-line);
}
.entry { margin-bottom: 0.8rem; }
.entry header { display: flex; align-items: center; gap: 0.45rem; margin-bottom: 0.35rem; font-size: 0.82rem; }
.entry header strong { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.entry time { color: var(--color-muted); font: 0.72rem var(--font-mono); }
.entry .pre { max-height: 260px; font-size: 0.74rem; }

.toasts { position: fixed; right: 1rem; bottom: 1rem; z-index: 70; display: grid; gap: 0.5rem; max-width: min(420px, calc(100vw - 2rem)); }
.toast {
  display: flex; align-items: flex-start; gap: 0.55rem; padding: 0.7rem 0.9rem; border-radius: 9px;
  background: var(--color-pine); color: #eaf3ee; font-size: 0.86rem; box-shadow: 0 10px 30px rgb(0 0 0 / 0.25);
  white-space: pre-wrap; overflow-wrap: anywhere;
}
.toast svg { margin-top: 2px; color: var(--color-mint); flex: none; }
.toast.error { background: #5a1f16; }
.toast.error svg { color: #ffb4a6; }
.scrim { display: none; }

@media (max-width: 1100px) {
  .activity { position: fixed; right: 0; z-index: 35; width: min(400px, 100vw); box-shadow: -10px 0 30px rgb(0 0 0 / 0.2); }
}
@media (max-width: 900px) {
  .side { position: fixed; left: 0; transform: translateX(-100%); transition: transform 0.18s ease; }
  .side.open { transform: none; box-shadow: 10px 0 30px rgb(0 0 0 / 0.3); }
  .scrim { display: block; position: fixed; inset: 0; z-index: 25; background: rgb(8 20 14 / 0.45); }
  .menu-btn { display: inline-flex; }
  .top { padding: 0 0.8rem; }
  .content { padding: 1.2rem 0.9rem 2.5rem; }
}
@media (max-width: 560px) { .hide-sm { display: none; } }
</style>
