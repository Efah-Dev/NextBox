package store

// Steam is split in two (v2.3): its store and community, which often cannot
// be reached from mainland China and want the proxy, and its downloads,
// which Steam serves from CDNs inside China and which want to go direct.
// The geosite "steam" list has both, so a rule on it sends both the same way.

// SteamStoreDomains are the store, the community, the client's login and
// their images (domain suffixes).
var SteamStoreDomains = []string{
	"steampowered.com", "steamcommunity.com", "steamstatic.com", "steamusercontent.com",
	"steam-chat.com", "steam-api.com", "steamgames.com", "steamserver.net", "s.team", "steam.tv",
	"steamdeck.com", "valvesoftware.com",
	"steamcdn-a.akamaihd.net", "steamcommunity-a.akamaihd.net", "steamstore-a.akamaihd.net",
	"steamuserimages-a.akamaihd.net", "steamusercontent-a.akamaihd.net", "steamvideo-a.akamaihd.net",
	"steambroadcast.akamaized.net", "steammobile.akamaized.net",
}

// SteamDownloadDomains are where games and client updates are downloaded
// from: Valve's content servers and the CDNs in mainland China that the
// client picks there (domain suffixes). They are checked before the store
// domains: steamcontent.com and steampipe are downloads only.
var SteamDownloadDomains = []string{
	"steamcontent.com", "steampipe.akamaized.net", "steampipe-kr.akamaized.net", "steampipe-partner.akamaized.net",
	"steampowered.com.8686c.com", "steamstatic.com.8686c.com", "dl.steam.clngaa.com",
	"st.dl.bscstorage.net", "st.dl.eccdnx.com", "st.dl.pinyuncloud.com", "xz.pphimalayanrt.com",
	"lv.queniujq.cn", "client-update.queniuqe.com", "steamchina.com", "wmsjsteam.com",
}

// Services of the catalog (Rule.App) for the two halves.
const (
	SteamStoreApp    = "steam"
	SteamDownloadApp = "steam-download"
)

// splitSteam turns a catalog rule of v2.2, which sent everything of Steam
// (geosite "steam") one way, into the two rules of v2.3 with the same
// target, so that nothing changes until the user picks another way for one
// of them.
func splitSteam(rules []Rule) []Rule {
	idx := -1
	for i, r := range rules {
		if r.App == SteamStoreApp && r.Kind == "geosite" && len(r.Values) == 1 && r.Values[0] == "steam" {
			idx = i
			break
		}
	}
	if idx < 0 {
		return rules
	}
	old := rules[idx]
	dl := Rule{Kind: "suffix", Values: append([]string(nil), SteamDownloadDomains...), Target: old.Target, Disabled: old.Disabled, App: SteamDownloadApp}
	st := Rule{Kind: "suffix", Values: append([]string(nil), SteamStoreDomains...), Target: old.Target, Disabled: old.Disabled, App: SteamStoreApp}
	out := make([]Rule, 0, len(rules)+1)
	out = append(out, rules[:idx]...)
	out = append(out, dl, st)
	return append(out, rules[idx+1:]...)
}
