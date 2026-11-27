import { createApp } from 'vue'
import App from './App.vue'
import '@fontsource-variable/noto-sans-sc'
import flagFont from 'country-flag-emoji-polyfill/dist/TwemojiCountryFlags.woff2?url'
import './style.css'
import './motion/motion.css'

// Windows has no glyphs for flag emoji (it shows "HK" instead of a flag), so
// a small font that only covers flags is put in front of the UI fonts.
const flags = document.createElement('style')
flags.textContent = `@font-face {
  font-family: 'Twemoji Country Flags';
  unicode-range: U+1F1E6-1F1FF, U+1F3F4, U+E0062-E0063, U+E0065, U+E0067, U+E006C, U+E006E, U+E0073-E0074, U+E0077, U+E007F;
  src: url('${flagFont}') format('woff2');
  font-display: block;
}`
document.head.appendChild(flags)

// A file dropped on the window must never be opened by the WebView itself
// (it would show in a window of its own). Wails stops that only on dragover;
// a quick drag can end before the first dragover, so dragenter and drop are
// stopped too, from the start. Wails still gets the drop (DropZone.vue).
for (const type of ['dragenter', 'dragover', 'drop']) {
  window.addEventListener(type, (e) => {
    const d = (e as DragEvent).dataTransfer
    if (!d?.types.includes('Files')) return
    e.preventDefault()
    if (type !== 'drop') d.dropEffect = 'copy'
  })
}

createApp(App).mount('#app')
