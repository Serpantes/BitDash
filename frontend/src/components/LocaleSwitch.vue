<template>
  <div class="locale-switch" role="group" :aria-label="t('app.language')">
    <button
      v-for="code in LOCALES"
      :key="code"
      type="button"
      class="locale-switch-btn"
      :class="{ active: locale === code }"
      :aria-pressed="locale === code"
      @click="setLocale(code)"
    >
      {{ code }}
    </button>
  </div>
</template>

<script>
import { LOCALES, locale, setLocale } from '../i18n'

export default {
  name: 'LocaleSwitch',
  setup() {
    return { LOCALES, locale, setLocale }
  },
}
</script>

<style scoped>
.locale-switch {
  display: inline-flex;
  padding: 2px;
  gap: 2px;
  border: 1px solid var(--frame-soft);
  border-radius: 3px;
  background: rgba(20, 18, 30, 0.35);
}
.locale-switch-btn {
  padding: 3px 7px;
  border: none;
  border-radius: 2px;
  background: transparent;
  color: var(--muted);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  cursor: pointer;
}
.locale-switch-btn:hover:not(.active) { color: var(--text); }
.locale-switch-btn.active { background: var(--cream); color: var(--on-cream); }
</style>
