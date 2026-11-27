<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import Icon from '../components/Icon.vue'
import PageHeader from '../components/PageHeader.vue'
import { t, lang } from '../i18n'
import type { User } from '../lib/api'
import { useLoad } from '../lib/useLoad'
import { run, state, loadSession, toast } from '../lib/state'
import { confirmAction } from '../lib/confirm'
import { api } from '../lib/api'
import { passkeyRegister, passkeysAvailable } from '../lib/passkey'

const { data: users, reload } = useLoad<User[]>(() => '/api/users')
const me = computed(() => state.session?.user ?? null)
const add = reactive({ username: '', password: '' })
const pw = reactive({ username: me.value?.username ?? '', password: '' })
const totp = ref<{ secret: string; uri: string } | null>(null)
const code = ref('')
const current = ref('')

async function refresh(): Promise<void> {
  await Promise.all([reload(), loadSession()])
}
async function addUser(): Promise<void> {
  if ((await run(`user add ${add.username}`, '/api/account/add', { ...add }, t('security.userAdded'))) !== null) {
    Object.assign(add, { username: '', password: '' })
    await reload()
  }
}
async function deleteUser(u: User): Promise<void> {
  if (!(await confirmAction(t('security.confirmDelete', { name: u.username })))) return
  if ((await run(`user delete ${u.username}`, '/api/account/delete', { username: u.username }, t('security.userDeleted'))) !== null) await reload()
}
async function setPassword(): Promise<void> {
  if ((await run(`password ${pw.username}`, '/api/account/password', { ...pw }, t('security.passwordUpdated'))) !== null) pw.password = ''
}
async function beginTotp(): Promise<void> {
  try {
    totp.value = await api.post<{ secret: string; uri: string }>('/api/account/totp-begin')
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
  }
}
async function confirmTotp(): Promise<void> {
  if ((await run('totp confirm', '/api/account/totp-confirm', { code: code.value }, t('security.totpEnabled'))) !== null) {
    totp.value = null
    code.value = ''
    await refresh()
  }
}
async function disableTotp(): Promise<void> {
  if (!(await confirmAction(t('security.confirmTotpDisable')))) return
  if ((await run('totp disable', '/api/account/totp-disable', { password: current.value }, t('security.totpDisabled'))) !== null) await refresh()
}
async function addPasskey(): Promise<void> {
  if (!passkeysAvailable()) {
    toast('error', t('auth.passkeyUnsupported'))
    return
  }
  try {
    await passkeyRegister()
    toast('ok', t('security.passkeyAdded'))
    await refresh()
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
  }
}
async function deletePasskey(id: string, name: string): Promise<void> {
  if (!(await confirmAction(t('security.confirmPasskeyDelete', { name })))) return
  if ((await run('passkey delete', '/api/account/passkey-delete', { id, password: current.value }, t('security.passkeyDeleted'))) !== null) await refresh()
}
async function copy(text: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text)
    toast('ok', t('common.copied'))
  } catch (e) {
    toast('error', e instanceof Error ? e.message : String(e))
  }
}
function date(s: string): string {
  return s ? new Date(s).toLocaleDateString(lang.value) : '—'
}
</script>

<template>
  <PageHeader :title="t('security.title')" :subtitle="t('security.subtitle')">
    <button class="btn" @click="refresh"><Icon name="refresh" /> {{ t('common.refresh') }}</button>
  </PageHeader>

  <section class="card mb-4 flex flex-wrap items-end gap-3 p-4">
    <label class="field min-w-[240px] flex-1">{{ t('security.currentPassword') }}
      <input v-model="current" class="input" type="password" autocomplete="current-password" />
      <span class="hint">{{ t('security.currentPasswordHint') }}</span>
    </label>
  </section>

  <div class="grid gap-4 xl:grid-cols-2">
    <section class="card overflow-hidden">
      <div class="card-head"><h2>{{ t('security.users') }}</h2></div>
      <table class="table">
        <thead><tr><th>{{ t('common.name') }}</th><th>{{ t('security.role') }}</th><th>2FA</th><th>{{ t('security.passkeys') }}</th><th /></tr></thead>
        <tbody>
          <tr v-for="u in users ?? []" :key="u.username">
            <td class="font-semibold">{{ u.username }} <span v-if="u.username === me?.username" class="pill">{{ t('security.you') }}</span></td>
            <td>{{ u.role }}</td>
            <td><Icon :name="u.totp ? 'check' : 'close'" :size="16" :class="u.totp ? 'text-leaf' : 'text-muted'" /></td>
            <td>{{ u.passkeys.length }}</td>
            <td class="text-right">
              <button v-if="u.username !== me?.username" class="btn btn-ghost btn-icon btn-sm btn-danger" :title="t('common.delete')" :aria-label="t('common.delete')" @click="deleteUser(u)"><Icon name="trash" /></button>
            </td>
          </tr>
        </tbody>
      </table>
      <form class="grid gap-3 border-t border-line p-4" @submit.prevent="addUser">
        <strong class="text-[0.9rem]">{{ t('security.addUser') }}</strong>
        <div class="form-grid">
          <label class="field">{{ t('auth.username') }}<input v-model="add.username" class="input" required autocomplete="off" spellcheck="false" /><span class="hint">{{ t('auth.usernameHint') }}</span></label>
          <label class="field">{{ t('auth.password') }}<input v-model="add.password" class="input" type="password" required autocomplete="new-password" /><span class="hint">{{ t('auth.passwordHint') }}</span></label>
        </div>
        <div><button class="btn btn-primary"><Icon name="plus" /> {{ t('security.addUser') }}</button></div>
      </form>
    </section>

    <section class="card">
      <div class="card-head"><h2>{{ t('security.password') }}</h2></div>
      <form class="grid gap-4 p-4" @submit.prevent="setPassword">
        <label class="field">{{ t('auth.username') }}
          <select v-model="pw.username" class="input"><option v-for="u in users ?? []" :key="u.username" :value="u.username">{{ u.username }}</option></select>
        </label>
        <label class="field">{{ t('auth.password') }}<input v-model="pw.password" class="input" type="password" required autocomplete="new-password" /><span class="hint">{{ t('auth.passwordHint') }}</span></label>
        <div><button class="btn btn-primary"><Icon name="key" /> {{ t('common.save') }}</button></div>
      </form>
    </section>

    <section class="card">
      <div class="card-head">
        <h2>{{ t('security.totp') }}</h2>
        <span class="pill" :class="me?.totp ? 'on' : ''">{{ me?.totp ? t('common.yes') : t('common.no') }}</span>
      </div>
      <div class="grid gap-4 p-4">
        <p class="m-0 text-[0.88rem] text-muted">{{ me?.totp ? t('security.totpOn') : t('security.totpOff') }}</p>
        <div v-if="me?.totp"><button class="btn btn-danger" @click="disableTotp">{{ t('security.totpDisable') }}</button></div>
        <template v-else-if="totp">
          <p class="m-0 text-[0.86rem]">{{ t('security.totpStep') }}</p>
          <div class="field">{{ t('security.totpSecret') }}
            <div class="flex gap-2"><code class="input mono flex items-center break-all select-all">{{ totp.secret }}</code><button class="btn" @click="copy(totp.secret)"><Icon name="copy" /></button></div>
          </div>
          <div class="field">{{ t('security.totpUri') }}
            <div class="flex gap-2"><code class="input mono flex items-center text-[0.74rem] break-all select-all">{{ totp.uri }}</code><button class="btn" @click="copy(totp.uri)"><Icon name="copy" /></button></div>
          </div>
          <form class="flex flex-wrap items-end gap-3" @submit.prevent="confirmTotp">
            <label class="field">{{ t('security.totpCode') }}<input v-model="code" class="input mono w-[140px]" inputmode="numeric" autocomplete="one-time-code" maxlength="8" required /></label>
            <button class="btn btn-primary"><Icon name="check" /> {{ t('security.totpConfirm') }}</button>
          </form>
        </template>
        <div v-else><button class="btn btn-primary" @click="beginTotp"><Icon name="shield" /> {{ t('security.totpBegin') }}</button></div>
      </div>
    </section>

    <section class="card">
      <div class="card-head"><div><h2>{{ t('security.passkeys') }}</h2><p class="m-0 text-[0.8rem] text-muted">{{ t('security.passkeysHint') }}</p></div></div>
      <div class="grid gap-3 p-4">
        <p v-if="!me?.passkeys.length" class="m-0 text-[0.88rem] text-muted">{{ t('security.noPasskey') }}</p>
        <div v-for="k in me?.passkeys ?? []" :key="k.id" class="flex items-center justify-between gap-3 rounded-lg border border-line px-3 py-2">
          <div class="min-w-0"><strong class="block truncate text-[0.88rem]">{{ k.name }}</strong><span class="text-[0.78rem] text-muted">{{ t('security.created', { date: date(k.createdAt) }) }}</span></div>
          <button class="btn btn-ghost btn-icon btn-sm btn-danger" :title="t('common.delete')" :aria-label="t('common.delete')" @click="deletePasskey(k.id, k.name)"><Icon name="trash" /></button>
        </div>
        <div><button class="btn btn-primary" @click="addPasskey"><Icon name="key" /> {{ t('security.passkeyAdd') }}</button></div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.pill.on { color: var(--color-leaf); }
</style>
