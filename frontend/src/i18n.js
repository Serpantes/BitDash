import { ref, watch } from 'vue'
import en from './locales/en'
import ru from './locales/ru'

const STORAGE_KEY = 'bitdash.locale'
const MESSAGES = { en, ru }
const INTL_LOCALES = { en: 'en-US', ru: 'ru-RU' }

export const LOCALES = Object.keys(MESSAGES)

function storedLocale() {
  try {
    const value = localStorage.getItem(STORAGE_KEY)
    if (value && MESSAGES[value]) return value
  } catch {}
  return 'en'
}

export const locale = ref(storedLocale())

watch(locale, (value) => {
  try {
    localStorage.setItem(STORAGE_KEY, value)
  } catch {}
  if (typeof document !== 'undefined') document.documentElement.lang = value
}, { immediate: true })

export function setLocale(value) {
  if (MESSAGES[value]) locale.value = value
}

export function intlLocale() {
  return INTL_LOCALES[locale.value] ?? 'en-US'
}

function lookup(dict, key) {
  let node = dict
  for (const part of key.split('.')) {
    if (node == null) return undefined
    node = node[part]
  }
  return node
}

export function t(key, params = {}) {
  const message = lookup(MESSAGES[locale.value], key) ?? lookup(en, key)
  if (message == null) return key
  if (typeof message === 'function') return message(params)
  if (typeof message !== 'string') return message
  return message.replace(/\{(\w+)\}/g, (match, name) => (params[name] ?? match))
}

export function formatNumber(value, options) {
  return Number(value).toLocaleString(intlLocale(), options)
}
