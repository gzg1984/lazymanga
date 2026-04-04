import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import './input.css'
import router from './router'
import App from './App.vue'
import "@lazycatcloud/lzc-file-pickers"

createApp(App).use(router).use(ElementPlus).mount('#app')
