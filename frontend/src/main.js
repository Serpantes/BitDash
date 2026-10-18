import { createApp } from 'vue'
import App from './App.vue'
import { t } from './i18n'
import { initPinnedSync } from './composables/usePinned'
import './styles.css'

const app = createApp(App)
app.config.globalProperties.t = t
initPinnedSync().finally(() => app.mount('#app'))
