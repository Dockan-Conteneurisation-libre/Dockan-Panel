// Application-wide state: the session, notifications and the output of the
// last commands (shown in the activity panel).
import { reactive } from 'vue'
import { api, setCsrf, setUnauthorizedHandler, type Session, type Output } from './api'

export const state = reactive({
  ready: false,
  session: null as Session | null,
  toasts: [] as { id: number; kind: 'ok' | 'error'; text: string }[],
  activity: [] as { id: number; title: string; ok: boolean; output: string; at: Date }[],
  activityOpen: false,
  busy: 0,
})

let seq = 0

setUnauthorizedHandler(() => {
  if (state.session?.user) void loadSession().catch((e: unknown) => toast("error", e instanceof Error ? e.message : String(e)))
})

export async function loadSession(): Promise<void> {
  const s = await api.get<Session>('/api/session')
  setCsrf(s.csrf)
  state.session = s
  state.ready = true
}

export function applySession(s: Session): void {
  setCsrf(s.csrf)
  state.session = s
}

export function toast(kind: 'ok' | 'error', text: string): void {
  const id = ++seq
  state.toasts.push({ id, kind, text })
  setTimeout(() => {
    const i = state.toasts.findIndex((t) => t.id === id)
    if (i >= 0) state.toasts.splice(i, 1)
  }, kind === 'error' ? 9000 : 4000)
}

function record(title: string, ok: boolean, output: string): void {
  state.activity.unshift({ id: ++seq, title, ok, output, at: new Date() })
  if (state.activity.length > 30) state.activity.pop()
}

/**
 * Runs an action, records its output in the activity panel and shows a
 * notification. Returns the output, or null when the action failed.
 */
export async function run(title: string, path: string, body: unknown, okText: string): Promise<string | null> {
  state.busy++
  try {
    const res = await api.post<Output>(path, body)
    record(title, true, res.output)
    toast('ok', okText)
    return res.output
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    record(title, false, msg)
    toast('error', msg.split('\n').slice(-3).join('\n'))
    return null
  } finally {
    state.busy--
  }
}
