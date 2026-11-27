// Well-known services and the geosite sets that match them: they name the site
// categories offered in the rules, and rules picked in the service catalog of
// 2.2–2.4 (rules with an app field) by the service's name.
// Brand names belong to their owners and only show which service a rule is about.
import { t } from './i18n'

export interface Service {
  id: string
  name: string
  sets: string[] // geosite names
}

export const services: Service[] = [
  { id: 'chatgpt', name: "ChatGPT", sets: ['openai'] },
  { id: 'claude', name: "Claude", sets: ['anthropic'] },
  { id: 'gemini', name: "Gemini", sets: ['google-gemini'] },
  { id: 'perplexity', name: "Perplexity", sets: ['perplexity'] },
  { id: 'copilot', name: "GitHub Copilot", sets: ['github-copilot'] },
  { id: 'poe', name: "Poe", sets: ['poe'] },
  { id: 'grok', name: "Grok", sets: ['xai'] },
  { id: 'youtube', name: "YouTube", sets: ['youtube'] },
  { id: 'netflix', name: "Netflix", sets: ['netflix'] },
  { id: 'disney', name: "Disney+", sets: ['disney'] },
  { id: 'primevideo', name: "Prime Video", sets: ['primevideo'] },
  { id: 'hbo', name: "HBO Max", sets: ['hbo'] },
  { id: 'tiktok', name: "TikTok", sets: ['tiktok'] },
  { id: 'spotify', name: "Spotify", sets: ['spotify'] },
  { id: 'twitch', name: "Twitch", sets: ['twitch'] },
  { id: 'telegram', name: "Telegram", sets: ['telegram'] },
  { id: 'twitter', name: "X (Twitter)", sets: ['twitter'] },
  { id: 'instagram', name: "Instagram", sets: ['instagram'] },
  { id: 'facebook', name: "Facebook", sets: ['facebook'] },
  { id: 'whatsapp', name: "WhatsApp", sets: ['whatsapp'] },
  { id: 'discord', name: "Discord", sets: ['discord'] },
  { id: 'reddit', name: "Reddit", sets: ['reddit'] },
  { id: 'line', name: "LINE", sets: ['line'] },
  { id: 'google', name: "Google", sets: ['google'] },
  { id: 'github', name: "GitHub", sets: ['github'] },
  { id: 'microsoft', name: "Microsoft", sets: ['microsoft'] },
  { id: 'apple', name: "Apple", sets: ['apple'] },
  { id: 'amazon', name: "Amazon", sets: ['amazon'] },
  { id: 'paypal', name: "PayPal", sets: ['paypal'] },
  { id: 'dropbox', name: "Dropbox", sets: ['dropbox'] },
  { id: 'notion', name: "Notion", sets: ['notion'] },
  { id: 'wikipedia', name: "Wikipedia", sets: ['wikimedia'] },
  { id: 'docker', name: "Docker", sets: ['docker'] },
  { id: 'steam', name: t("Steam 商店 / 社区"), sets: [] },
  { id: 'steam-download', name: t("Steam 下载"), sets: [] },
  { id: 'epic', name: "Epic Games", sets: ['epicgames'] },
  { id: 'playstation', name: "PlayStation", sets: ['playstation'] },
  { id: 'xbox', name: "Xbox", sets: ['xbox'] },
  { id: 'nintendo', name: "Nintendo", sets: ['nintendo'] },
  { id: 'blizzard', name: "Battle.net", sets: ['blizzard'] },
  { id: 'ea', name: "EA", sets: ['ea'] },
  { id: 'bilibili', name: t("哔哩哔哩"), sets: ['bilibili'] },
  { id: 'tencent', name: t("微信 / QQ"), sets: ['tencent'] },
  { id: 'alibaba', name: t("淘宝 / 支付宝"), sets: ['alibaba'] },
  { id: 'douyin', name: t("抖音"), sets: ['douyin'] },
  { id: 'netease', name: t("网易 / 网易云音乐"), sets: ['netease'] },
  { id: 'baidu', name: t("百度"), sets: ['baidu'] },
  { id: 'xiaohongshu', name: t("小红书"), sets: ['xiaohongshu'] },
  { id: 'iqiyi', name: t("爱奇艺"), sets: ['iqiyi'] },
  { id: 'zhihu', name: t("知乎"), sets: ['zhihu'] },
  { id: 'jd', name: t("京东"), sets: ['jd'] },
]

export const serviceOf = (id: string) => services.find((s) => s.id === id)
