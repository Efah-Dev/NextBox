// Pinyin initials of Chinese text ("日本" → "rb"), for searching lists by
// 简拼. No dictionary: the browser's Chinese collation sorts characters by
// pinyin, so the first character of each initial marks where it starts.
// Rare characters with several readings may get the other one.

const letters = 'abcdefghjklmnopqrstwxyz'
const firsts = '阿八嚓哒妸发旮哈讥咔垃妈拏噢妑七呥仨他屲夕丫帀'
let collator: Intl.Collator | undefined

function initial(ch: string): string {
  if (!/\p{Script=Han}/u.test(ch)) return /[a-z0-9]/i.test(ch) ? ch.toLowerCase() : ''
  collator ??= new Intl.Collator('zh-Hans-CN')
  for (let i = firsts.length - 1; i >= 0; i--) {
    if (collator.compare(ch, firsts[i]) >= 0) return letters[i]
  }
  return ''
}

/** The initials of each character, e.g. "新加坡" → "xjp". */
export function pinyinInitials(text: string): string {
  return [...text].map(initial).join('')
}
