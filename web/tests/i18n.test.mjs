// Parity of the four dictionaries: same keys, no empty value, the same
// {placeholders}, and no text left in English in the other languages.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const dir = new URL('../src/i18n/', import.meta.url)
const entry = /^\s*'([a-zA-Z0-9.]+)':\s*(['"])((?:\\.|(?!\2).)*)\2,?\s*$/

function load(lang) {
  const map = new Map()
  for (const line of readFileSync(new URL(`${lang}.ts`, dir), 'utf8').split('\n')) {
    const m = line.match(entry)
    if (m) {
      assert.ok(!map.has(m[1]), `${lang}: duplicate key ${m[1]}`)
      map.set(m[1], m[3])
    }
  }
  return map
}

const placeholders = (s) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort().join(',')
const en = load('en')
// Values that are the same in every language: names, technical terms.
const same = new Set([
  'app.name', 'common.error', 'nav.images', 'nav.volumes', 'nav.stacks', 'overview.images', 'overview.volumes',
  'overview.stacks', 'containers.pid', 'containers.ports', 'run.isolation', 'stacks.yaml', 'images.title', 'images.tag',
  'volumes.title', 'stacks.title', 'system.dockan', 'detail.terminal', 'containers.image', 'run.image', 'run.ports',
  'run.volumes', 'nav.store', 'store.title', 'system.version', 'common.no', 'system.packages', 'system.mode',
  'security.totpCode', 'security.totpSecret', 'system.panel', 'common.actions', 'store.requires', 'overview.doctorHint',
  'system.arch', 'store.port', 'store.siteZip',
])

test('English has keys', () => assert.ok(en.size > 300))

for (const lang of ['eo', 'fr', 'es']) {
  test(`${lang} matches the English keys and placeholders`, () => {
    const d = load(lang)
    assert.deepEqual([...d.keys()].sort(), [...en.keys()].sort())
    for (const [k, v] of d) {
      assert.ok(v.trim() !== '', `${lang}: empty ${k}`)
      assert.equal(placeholders(v), placeholders(en.get(k)), `${lang}: placeholders of ${k}`)
      assert.ok(!/\\$/.test(v), `${lang}: trailing backslash in ${k}`)
    }
  })
  test(`${lang} has no untranslated sentence`, () => {
    const d = load(lang)
    const left = [...d].filter(([k, v]) => v === en.get(k) && /\s/.test(v) && !same.has(k) && !/^[a-z]/.test(v))
    assert.deepEqual(left.map(([k]) => k), [])
  })
}

test('Esperanto has no French or Spanish leak', () => {
  const words = /(?<!\p{L})(le|les|des|une|avec|pour|est|los|las|una|para|con|del)(?!\p{L})/iu
  for (const [k, v] of load('eo')) {
    assert.ok(!words.test(v.replace(/dockan [a-z -]+/gi, '')), `eo: foreign word in ${k}: ${v}`)
  }
})
