// Client of the panel's JSON API. Every write request carries the session
// CSRF token; errors are thrown with the server's message.

export interface Container {
  store: string
  name: string
  status: string
  pid: string
  image: string
  ports: string
}
export interface Image { tag: string; name: string; path: string }
export interface Volume { name: string; path: string }
export interface Backup { file: string; name: string; size: number; created: string }
export interface Network { name: string; driver: string; subnet: string; bridge: string }
export interface Stack { name: string; file: string; yaml: string; requiredImages: string[]; registryDir: string }
export interface Passkey { id: string; name: string; createdAt: string }
export interface User { username: string; role: string; totp: boolean; passkeys: Passkey[] }
export interface Session { setup: boolean; csrf: string; version: string; user: User | null }
export interface StoreAppRow { id: string; name: string; status: string; containers: string[]; file: string; target: string }
export interface StoreApp {
  id: string
  name: string
  category: string
  summary: string
  default_port?: number
  requires: string[]
  target: string
  installed: boolean
  autostart: boolean
  images: 'current' | 'available' | 'unknown'
  config: string
}
export interface StoreState { ready: boolean; apps: StoreApp[]; path: string; base: string; catalog: string; root: boolean }
export interface Overview {
  containers: number
  running: number
  images: number
  volumes: number
  networks: number
  stacks: number
  recent: Container[]
  doctor: string
  dockanVersion: string
}
export interface SystemInfo {
  panelVersion: string
  dockanVersion: string
  dockanPath: string
  user: string
  root: boolean
  automation: 'root' | 'sudo' | 'disabled'
  updateDir: string
  storage: string
  arch: string
  doctor: string
}
export interface Output { output: string }

export class ApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message)
  }
}

let csrf = ''
let onUnauthorized: (() => void) | null = null

/** Called when the server answers 401: the session expired or was closed. */
export function setUnauthorizedHandler(fn: () => void): void {
  onUnauthorized = fn
}

export function setCsrf(token: string): void {
  csrf = token
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, credentials: 'same-origin', headers: {} as Record<string, string> }
  const headers = init.headers as Record<string, string>
  if (method !== 'GET') headers['X-CSRF-Token'] = csrf
  if (body instanceof FormData) {
    init.body = body
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }
  const res = await fetch(path, init)
  const text = await res.text()
  let data: unknown = null
  if (text !== '') {
    try {
      data = JSON.parse(text)
    } catch {
      throw new ApiError(text.slice(0, 300), res.status)
    }
  }
  if (!res.ok) {
    if (res.status === 401) onUnauthorized?.()
    const message = (data as { error?: string } | null)?.error ?? res.statusText
    throw new ApiError(message, res.status)
  }
  return data as T
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T = Output>(path: string, body: unknown = {}) => request<T>('POST', path, body),
}
