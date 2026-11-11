// Renders docs/preview.png (Chinese) and docs/preview-en.png (English), the
// README hero images, with headless Chrome or Edge.
//
//   cd frontend && npm install && npm run build && cd ..
//   node docs/render/render.mjs
//
// The page (preview.html) shows the real frontend from frontend/dist twice,
// driven by a fake backend (mock.js) instead of the Go application.
import { spawn } from 'node:child_process'
import fs from 'node:fs'
import http from 'node:http'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const docs = path.dirname(here)
const dist = path.join(path.dirname(docs), 'frontend', 'dist')
if (!fs.existsSync(path.join(dist, 'index.html'))) throw new Error('frontend/dist is missing: run "npm run build" in frontend first')

const browser = [
  process.env.BROWSER,
  path.join(process.env.ProgramFiles ?? '', 'Google/Chrome/Application/chrome.exe'),
  path.join(process.env['ProgramFiles(x86)'] ?? '', 'Microsoft/Edge/Application/msedge.exe'),
  path.join(process.env.ProgramFiles ?? '', 'Microsoft/Edge/Application/msedge.exe'),
  '/usr/bin/google-chrome',
  '/usr/bin/chromium',
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
].find((p) => p && fs.existsSync(p))
if (!browser) throw new Error('Chrome or Edge is required (or set BROWSER to a Chromium executable)')

const types = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.png': 'image/png', '.svg': 'image/svg+xml' }

// "/" is the app with the mock backend in front of its bundle, "/render/…"
// are the files next to this script, everything else comes from frontend/dist.
const server = http.createServer((req, res) => {
  const url = new URL(req.url, 'http://localhost')
  let file
  let body
  if (url.pathname === '/') {
    body = fs.readFileSync(path.join(dist, 'index.html'), 'utf8').replace('<script', '<script src="/render/mock.js"></script>\n    <script')
    file = 'index.html'
  } else if (url.pathname.startsWith('/render/')) {
    file = path.join(here, path.basename(url.pathname))
  } else {
    file = path.join(dist, path.normalize(url.pathname))
    if (!file.startsWith(dist)) file = ''
  }
  if (body === undefined) {
    if (!file || !fs.existsSync(file) || !fs.statSync(file).isFile()) {
      res.writeHead(404).end()
      return
    }
    body = fs.readFileSync(file)
  }
  res.writeHead(200, { 'Content-Type': types[path.extname(file)] ?? 'application/octet-stream' }).end(body)
})
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
const base = `http://127.0.0.1:${server.address().port}/render/preview.html`

// a profile of its own, so that a browser that is already open is left alone
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'nextbox-render-'))
try {
  for (const [name, query] of [['preview.png', ''], ['preview-en.png', '?lang=en']]) {
    const out = path.join(docs, name)
    fs.rmSync(out, { force: true })
    // not spawnSync: that would block the server above
    await run(browser, [
      '--headless=new', '--disable-gpu', '--hide-scrollbars', '--force-device-scale-factor=1', '--no-first-run',
      `--user-data-dir=${profile}`, '--window-size=1800,1040', '--virtual-time-budget=6000', `--screenshot=${out}`, base + query,
    ])
    if (!fs.existsSync(out)) throw new Error(`${name} was not written`)
    console.log(`${name}  ${(fs.statSync(out).size / 1024).toFixed(0)} KB`)
  }
} finally {
  server.close()
  fs.rmSync(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 })
}

function run(cmd, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(cmd, args, { stdio: 'ignore' })
    child.on('error', reject)
    child.on('exit', resolve)
  })
}
