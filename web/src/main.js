import { createApp } from 'vue'
import { nanocatZhCN, setNanocatLocale } from 'nanocat-ui'
import App from './App.vue'
import router from './router'
import { applyThemeMode, getStoredThemeMode } from './lib/theme'
import './style.css'
import './styles/features.css'

setNanocatLocale(nanocatZhCN)
applyThemeMode(getStoredThemeMode())

createApp(App).use(router).mount('#app')
