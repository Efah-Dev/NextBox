// Brightness of the light theme and depth of the dark theme (设置 → 外观).
// Each level moves the surface colours (background, cards, lines, inputs) and
// the text between fixed end points, so contrast stays readable at every
// step. The defaults leave style.css as it is.

type RGB = [number, number, number]

interface Palette {
  base: RGB // page background, before the accent tint
  card: RGB
  panel: RGB // side panels, drawn translucent
  line: RGB
  inputBg: RGB
  inputBorder: RGB
  text: RGB
  text2: RGB
  text3: RGB
}

const hex = (h: string): RGB => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16)) as RGB

const pal = (p: Record<keyof Palette, string>): Palette =>
  Object.fromEntries(Object.entries(p).map(([k, v]) => [k, hex(v)])) as unknown as Palette

// light: 0 = near white, LIGHT_DEFAULT = the classic theme, 100 = a deep grey
// for the night (v2.1 scale, see store.ThemeScale)
export const LIGHT_DEFAULT = 20
const lightWhite = pal({
  base: '#fcfdfe', card: '#ffffff', panel: '#ffffff', line: '#dfe5ee', inputBg: '#ffffff',
  inputBorder: '#cfd9e8', text: '#18222f', text2: '#566579', text3: '#8f9cb0',
})
const lightBright = pal({
  base: '#f4f6fa', card: '#ffffff', panel: '#ffffff', line: '#e3e9f2', inputBg: '#ffffff',
  inputBorder: '#d5dfee', text: '#1d2a3b', text2: '#5a6a80', text3: '#95a2b5',
})
const lightDim = pal({
  base: '#c3c8d1', card: '#dde1e7', panel: '#d6dae1', line: '#c3cad5', inputBg: '#e4e7ec',
  inputBorder: '#b3bccb', text: '#0f1826', text2: '#45536a', text3: '#6f7b8f',
})

// dark: 0 = a light slate, 60 = the classic dark theme, 100 = all black (OLED)
export const DARK_DEFAULT = 60
const darkSoft = pal({
  base: '#272d39', card: '#323a4a', panel: '#323a4a', line: '#404a5e', inputBg: '#2b3241',
  inputBorder: '#4a556c', text: '#e9edf4', text2: '#aab5c7', text3: '#7c889d',
})
const darkClassic = pal({
  base: '#121722', card: '#1b2230', panel: '#1b2230', line: '#2a3344', inputBg: '#151b27',
  inputBorder: '#313b4e', text: '#e3e9f3', text2: '#9eabbf', text3: '#66738a',
})
const darkBlack = pal({
  base: '#000000', card: '#000000', panel: '#000000', line: '#1a1d24', inputBg: '#000000',
  inputBorder: '#262a33', text: '#d9dfe8', text2: '#929daf', text3: '#5a6475',
})

function mix(a: Palette, b: Palette, t: number): Palette {
  const out = {} as Palette
  for (const k of Object.keys(a) as (keyof Palette)[]) {
    out[k] = a[k].map((v, i) => Math.max(0, Math.min(255, Math.round(v + (b[k][i] - v) * t)))) as RGB
  }
  return out
}

// ---- OKLab, to change how light a colour is without changing its tint
type Lab = [number, number, number]
const lin = (v: number) => ((v /= 255) <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)
const gam = (v: number) => 255 * (v <= 0.0031308 ? 12.92 * v : 1.055 * v ** (1 / 2.4) - 0.055)
function toLab([r, g, b]: RGB): Lab {
  ;[r, g, b] = [lin(r), lin(g), lin(b)]
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b)
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b)
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b)
  return [
    0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
    1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  ]
}
function fromLab([L, a, b]: Lab): RGB {
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3
  const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3
  return [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  ].map((v) => Math.max(0, Math.min(255, Math.round(gam(Math.max(0, Math.min(1, v))))))) as RGB
}

/**
 * Moves each colour of the classic palette to the lightness of the same
 * colour in end, a share t of the way, keeping the tint of the classic theme:
 * the hue stays, and the colourfulness shrinks only as the colour nears white
 * or black (as when mixing with white or black). So the slider only makes the
 * interface lighter or darker, it does not turn it bluer or greyer.
 */
function shade(classic: Palette, end: Palette, t: number): Palette {
  const out = {} as Palette
  for (const k of Object.keys(classic) as (keyof Palette)[]) {
    const [L0, a, b] = toLab(classic[k])
    const L = Math.max(0, Math.min(1, L0 + (toLab(end[k])[0] - L0) * t))
    const f = L >= L0 ? (L0 >= 1 ? 0 : (1 - L) / (1 - L0)) : L0 <= 0 ? 0 : L / L0
    out[k] = fromLab([L, a * f, b * f])
  }
  return out
}

// the far ends reach 1.55 times as far as in v2.0, in the same direction,
// so that levels converted from the old scale keep their colours
const lightDeep = mix(lightBright, lightDim, 1.55)
const darkLight = mix(darkClassic, darkSoft, 1.55)

const rgb = (c: RGB) => `rgb(${c.join(', ')})`
const rgba = (c: RGB, a: number) => `rgba(${c.join(', ')}, ${a})`

/** The palette for a theme and level, or null for the stock colours. */
export function palette(dark: boolean, level: number): Palette | null {
  level = Math.max(0, Math.min(100, level || 0))
  if (!dark) {
    if (level === LIGHT_DEFAULT) return null
    return level < LIGHT_DEFAULT
      ? shade(lightBright, lightWhite, 1 - level / LIGHT_DEFAULT)
      : shade(lightBright, lightDeep, (level - LIGHT_DEFAULT) / (100 - LIGHT_DEFAULT))
  }
  if (level === DARK_DEFAULT) return null
  return level < DARK_DEFAULT
    ? shade(darkClassic, darkLight, 1 - level / DARK_DEFAULT)
    : shade(darkClassic, darkBlack, (level - DARK_DEFAULT) / (100 - DARK_DEFAULT))
}

const VARS = ['--bg', '--card', '--panel', '--line', '--input-bg', '--input-border', '--text', '--text-2', '--text-3']

/** CSS variables for a palette (also used for the previews in the settings). */
export function paletteVars(p: Palette, dark: boolean): Record<string, string> {
  return {
    '--bg': dark ? rgb(p.base) : `color-mix(in srgb, var(--accent) 6%, ${rgb(p.base)})`,
    '--card': rgb(p.card),
    '--panel': rgba(p.panel, dark ? 0.88 : 0.86),
    '--line': rgb(p.line),
    '--input-bg': rgb(p.inputBg),
    '--input-border': rgb(p.inputBorder),
    '--text': rgb(p.text),
    '--text-2': rgb(p.text2),
    '--text-3': rgb(p.text3),
  }
}

/** Applies the level to the page (dark is the theme in effect). */
export function applyLevel(el: HTMLElement, dark: boolean, level: number) {
  const p = palette(dark, level)
  if (!p) {
    VARS.forEach((v) => el.style.removeProperty(v))
    return
  }
  for (const [k, v] of Object.entries(paletteVars(p, dark))) el.style.setProperty(k, v)
}
