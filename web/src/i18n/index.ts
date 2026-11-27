// Four-language dictionary without dependency. Esperanto is the default;
// the choice is shared with the Dockan site through localStorage.
import { ref } from 'vue'
import { en, type Key } from './en'
import { eo } from './eo'
import { fr } from './fr'
import { es } from './es'

export type Lang = 'eo' | 'fr' | 'en' | 'es'
export const langs: { code: Lang; label: string }[] = [
  { code: 'eo', label: 'Esperanto' },
  { code: 'fr', label: 'Français' },
  { code: 'en', label: 'English' },
  { code: 'es', label: 'Español' },
]

const dicts: Record<Lang, Record<Key, string>> = { eo, fr, en, es }
const KEY = 'dockan-lang'

function initial(): Lang {
  try {
    const saved = localStorage.getItem(KEY)
    if (saved === 'eo' || saved === 'fr' || saved === 'en' || saved === 'es') return saved
  } catch {
    // storage unavailable: keep the default
  }
  return 'eo'
}

export const lang = ref<Lang>(initial())
document.documentElement.lang = lang.value

export function setLang(l: Lang): void {
  lang.value = l
  document.documentElement.lang = l
  try {
    localStorage.setItem(KEY, l)
  } catch {
    // storage unavailable: the choice lasts for this page only
  }
}

/** Translates a key; {name} placeholders are replaced from vars. */
export function t(key: Key, vars: Record<string, string | number> = {}): string {
  const text = dicts[lang.value][key] ?? en[key]
  return text.replace(/\{(\w+)\}/g, (_, k: string) => String(vars[k] ?? ''))
}

export type { Key }
