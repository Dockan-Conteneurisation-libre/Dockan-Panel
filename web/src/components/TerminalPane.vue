<script setup lang="ts">
// Interactive shell in a container: PTY on the server, long-polled output,
// keystrokes posted back. xterm.js renders it locally (bundled, no CDN).
import { onBeforeUnmount, ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import Icon from './Icon.vue'
import { t } from '../i18n'
import { api } from '../lib/api'
import { toast } from '../lib/state'

const props = defineProps<{ name: string; store: string }>()
const host = ref<HTMLDivElement | null>(null)
const connected = ref(false)
let term: Terminal | null = null
let fit: FitAddon | null = null
let id = ''
let offset = 0
let alive = false
let observer: ResizeObserver | null = null
let pending = ''
let flushTimer = 0

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

async function connect(): Promise<void> {
  if (!host.value) return
  term = new Terminal({
    cursorBlink: true, fontSize: 13, convertEol: false, scrollback: 5000,
    fontFamily: 'ui-monospace, "Cascadia Mono", "Liberation Mono", Menlo, monospace',
    theme: { background: '#0b1a13', foreground: '#dce9e1', cursor: '#9fd8c6', selectionBackground: '#2b8c7e66' },
  })
  fit = new FitAddon()
  term.loadAddon(fit)
  term.open(host.value)
  fit.fit()
  try {
    const res = await api.post<{ id: string }>('/api/terminal/start', { name: props.name, store: props.store })
    id = res.id
  } catch (e) {
    term.writeln(message(e))
    return
  }
  offset = 0
  alive = true
  connected.value = true
  term.onData(send)
  observer = new ResizeObserver(() => resize())
  observer.observe(host.value)
  resize()
  term.focus()
  void poll()
}

function send(data: string): void {
  pending += data
  if (!flushTimer) flushTimer = window.setTimeout(flush, 15)
}

async function flush(): Promise<void> {
  flushTimer = 0
  const data = pending
  pending = ''
  if (!data || !alive) return
  try {
    await api.post('/api/terminal/input', { id, data })
  } catch (e) {
    toast('error', message(e))
  }
}

async function resize(): Promise<void> {
  if (!term || !fit || !alive) return
  fit.fit()
  try {
    await api.post('/api/terminal/resize', { id, cols: term.cols, rows: term.rows })
  } catch (e) {
    toast('error', message(e))
  }
}

async function poll(): Promise<void> {
  while (alive && term) {
    try {
      const r = await api.post<{ data: string; offset: number; alive: boolean }>('/api/terminal/read', { id, offset })
      if (r.data) term.write(r.data)
      offset = r.offset
      if (!r.alive) {
        term.writeln(`\r\n\x1b[2m${t('term.closed')}\x1b[0m`)
        stop(false)
      }
    } catch (e) {
      term.writeln(`\r\n${message(e)}`)
      stop(false)
    }
  }
}

function stop(remote = true): void {
  if (remote && alive && id) {
    void api.post('/api/terminal/stop', { id }).catch((e: unknown) => toast('error', message(e)))
  }
  alive = false
  connected.value = false
  observer?.disconnect()
  observer = null
}

function close(): void {
  stop()
  term?.dispose()
  term = null
}

onBeforeUnmount(close)
</script>

<template>
  <div class="grid gap-3">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <p class="m-0 text-[0.84rem] text-muted">{{ t('term.hint') }}</p>
      <button v-if="!connected" class="btn btn-primary btn-sm" @click="close(); connect()"><Icon name="terminal" /> {{ t('term.connect') }}</button>
      <button v-else class="btn btn-sm" @click="close"><Icon name="close" /> {{ t('term.disconnect') }}</button>
    </div>
    <div ref="host" class="term" />
  </div>
</template>

<style scoped>
.term { height: min(62vh, 560px); padding: 8px; border-radius: 9px; background: #0b1a13; border: 1px solid var(--color-pine-line); overflow: hidden; }
</style>
