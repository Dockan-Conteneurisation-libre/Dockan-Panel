// WebAuthn helpers: the server sends base64url strings, the browser wants bytes.
import { api, type Session } from './api'

export function passkeysAvailable(): boolean {
  return window.isSecureContext && typeof window.PublicKeyCredential === 'function'
}

function fromB64(value: string): Uint8Array<ArrayBuffer> {
  const b64 = value.replace(/-/g, '+').replace(/_/g, '/')
  const padded = b64 + '='.repeat((4 - (b64.length % 4)) % 4)
  return Uint8Array.from(atob(padded), (c) => c.charCodeAt(0))
}

function toB64(buf: ArrayBuffer | null): string {
  if (!buf) return ''
  let s = ''
  for (const b of new Uint8Array(buf)) s += String.fromCharCode(b)
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

interface CredentialRef { type: 'public-key'; id: string }
interface GetOptions { challenge: string; allowCredentials: CredentialRef[]; userVerification: UserVerificationRequirement; timeout: number; rpId: string }
interface CreateOptions {
  challenge: string
  rp: { name: string; id: string }
  user: { id: string; name: string; displayName: string }
  pubKeyCredParams: PublicKeyCredentialParameters[]
  authenticatorSelection: AuthenticatorSelectionCriteria
  timeout: number
  attestation: AttestationConveyancePreference
}

export async function passkeyLogin(username: string): Promise<Session> {
  const { publicKey: o } = await api.post<{ publicKey: GetOptions }>('/api/passkeys/login-options', { username })
  const cred = (await navigator.credentials.get({
    publicKey: {
      challenge: fromB64(o.challenge),
      allowCredentials: o.allowCredentials.map((c) => ({ type: c.type, id: fromB64(c.id) })),
      userVerification: o.userVerification,
      timeout: o.timeout,
      rpId: o.rpId,
    },
  })) as PublicKeyCredential | null
  if (!cred) throw new Error('no passkey')
  const r = cred.response as AuthenticatorAssertionResponse
  return api.post<Session>('/api/passkeys/login-verify', {
    id: cred.id,
    clientDataJSON: toB64(r.clientDataJSON),
    authenticatorData: toB64(r.authenticatorData),
    signature: toB64(r.signature),
  })
}

export async function passkeyRegister(): Promise<void> {
  const { publicKey: o } = await api.post<{ publicKey: CreateOptions }>('/api/passkeys/register-options')
  const cred = (await navigator.credentials.create({
    publicKey: {
      challenge: fromB64(o.challenge),
      rp: o.rp,
      user: { id: fromB64(o.user.id), name: o.user.name, displayName: o.user.displayName || o.user.name },
      pubKeyCredParams: o.pubKeyCredParams,
      authenticatorSelection: o.authenticatorSelection,
      timeout: o.timeout,
      attestation: o.attestation,
    },
  })) as PublicKeyCredential | null
  if (!cred) throw new Error('no passkey')
  const r = cred.response as AuthenticatorAttestationResponse
  await api.post('/api/passkeys/register-verify', {
    id: cred.id,
    clientDataJSON: toB64(r.clientDataJSON),
    publicKey: toB64(r.getPublicKey()),
  })
}
