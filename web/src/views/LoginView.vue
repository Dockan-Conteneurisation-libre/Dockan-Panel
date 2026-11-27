<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '../components/Icon.vue'
import LangSelect from '../components/LangSelect.vue'
import { t } from '../i18n'
import { api, type Session } from '../lib/api'
import { applySession, state } from '../lib/state'
import { passkeyLogin, passkeysAvailable } from '../lib/passkey'

const setup = computed(() => state.session?.setup === true)
const username = ref('')
const password = ref('')
const confirm = ref('')
const token = ref('')
const totp = ref('')
const error = ref('')
const busy = ref(false)

async function submit(): Promise<void> {
  error.value = ''
  if (setup.value && password.value !== confirm.value) {
    error.value = t('auth.mismatch')
    return
  }
  busy.value = true
  try {
    const s = setup.value
      ? await api.post<Session>('/api/setup', { username: username.value, password: password.value, confirm: confirm.value, token: token.value.trim() })
      : await api.post<Session>('/api/login', { username: username.value, password: password.value, totp: totp.value })
    applySession(s)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
    password.value = ''
    confirm.value = ''
    totp.value = ''
  }
}

async function withPasskey(): Promise<void> {
  error.value = ''
  if (!passkeysAvailable()) {
    error.value = t('auth.passkeyUnsupported')
    return
  }
  if (!username.value.trim()) {
    error.value = t('auth.passkeyNeedUser')
    return
  }
  busy.value = true
  try {
    applySession(await passkeyLogin(username.value.trim()))
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="login">
    <section class="art" aria-hidden="true">
      <div class="art-inner">
        <img src="/dockan-logo.svg" alt="" width="64" height="64" />
        <h2>{{ t('app.name') }}</h2>
        <p>{{ t('app.tagline') }}</p>
        <pre class="art-code">$ dockan ps -a --scope all
STORE   NAME      STATUS
user    web       running
system  forgejo   running</pre>
      </div>
    </section>
    <section class="form-side">
      <div class="flex justify-end"><LangSelect /></div>
      <form class="mx-auto mt-10 grid w-full max-w-[380px] gap-4" @submit.prevent="submit">
        <div>
          <h1 class="m-0 text-[1.5rem] font-[680]">{{ setup ? t('auth.setupTitle') : t('auth.loginTitle') }}</h1>
          <p class="mt-1 mb-0 text-[0.9rem] text-muted">{{ setup ? t('auth.setupIntro') : t('auth.loginIntro') }}</p>
        </div>
        <label v-if="setup" class="field">
          {{ t('auth.setupToken') }}
          <input v-model="token" class="input mono" autocomplete="off" spellcheck="false" required />
          <span class="hint">{{ t('auth.setupTokenHint') }}</span>
        </label>
        <label class="field">
          {{ t('auth.username') }}
          <input v-model="username" class="input" autocomplete="username" autocapitalize="none" spellcheck="false" required />
          <span v-if="setup" class="hint">{{ t('auth.usernameHint') }}</span>
        </label>
        <label class="field">
          {{ t('auth.password') }}
          <input v-model="password" class="input" type="password" :autocomplete="setup ? 'new-password' : 'current-password'" required />
          <span v-if="setup" class="hint">{{ t('auth.passwordHint') }}</span>
        </label>
        <label v-if="setup" class="field">
          {{ t('auth.confirm') }}
          <input v-model="confirm" class="input" type="password" autocomplete="new-password" required />
        </label>
        <label v-else class="field">
          {{ t('auth.totp') }} <span class="hint">({{ t('common.optional') }})</span>
          <input v-model="totp" class="input" inputmode="numeric" autocomplete="one-time-code" maxlength="8" />
          <span class="hint">{{ t('auth.totpHint') }}</span>
        </label>
        <p v-if="error" class="m-0 rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-[0.86rem] text-danger" role="alert">{{ error }}</p>
        <button class="btn btn-primary w-full" :disabled="busy">{{ setup ? t('auth.create') : t('auth.signIn') }}</button>
        <button v-if="!setup" type="button" class="btn w-full" :disabled="busy" @click="withPasskey">
          <Icon name="key" /> {{ t('auth.passkey') }}
        </button>
      </form>
    </section>
  </div>
</template>

<style scoped>
.login { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); min-height: 100vh; }
.art {
  display: grid; place-items: center; padding: 3rem; color: #e2ebe5;
  background:
    radial-gradient(circle at 20% 15%, rgb(43 140 126 / 0.35), transparent 45%),
    radial-gradient(circle at 85% 90%, rgb(26 94 58 / 0.5), transparent 50%),
    var(--color-pine);
}
.art-inner { max-width: 420px; }
.art img { border-radius: 14px; }
.art h2 { margin: 1.2rem 0 0.3rem; font-size: 2rem; color: #fff; letter-spacing: -0.02em; }
.art p { margin: 0; color: var(--color-mint); font-size: 1.05rem; }
.art-code {
  margin: 2rem 0 0; padding: 1rem 1.1rem; border-radius: 10px; background: rgb(0 0 0 / 0.28);
  border: 1px solid var(--color-pine-line); color: #cfe3d8; font: 0.8rem/1.7 var(--font-mono); white-space: pre; overflow: auto;
}
.form-side { padding: 1.2rem 1.4rem 3rem; background: var(--color-paper); }
@media (max-width: 860px) {
  .login { grid-template-columns: 1fr; }
  .art { display: none; }
}
</style>
