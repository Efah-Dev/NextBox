package store

import (
	"net"
	"strconv"
	"strings"
)

// Profile is one proxy node. The node itself is stored as a sing-box outbound
// object (without "tag"), so new protocols and fields need no model changes.
type Profile struct {
	ID       int64          `json:"id"`
	GroupID  int64          `json:"groupId"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Outbound map[string]any `json:"outbound"`

	// Latency in ms: 0 = not tested, -1 = failed.
	Latency      int    `json:"latency"`
	LatencyError string `json:"latencyError,omitempty"`
	LatencyKind  string `json:"latencyKind,omitempty"` // how it was measured: "tcp" or "url"
	TestedAt     int64  `json:"testedAt,omitempty"`
	Upload       int64  `json:"upload"`
	Download     int64  `json:"download"`

	// Chain lists the member nodes of a chain proxy by ID, entry first and
	// exit last. Type is TypeChain and Outbound is empty for such a profile.
	Chain []int64 `json:"chain,omitempty"`

	// Where the node is, as last detected: an ISO 3166 code ("JP"), the IP it
	// was looked up from, which IP that was (CountryExit or CountryServer,
	// see Settings.CountrySource) and when.
	Country       string `json:"country,omitempty"`
	CountryIP     string `json:"countryIp,omitempty"`
	CountrySource string `json:"countrySource,omitempty"`
	CountryAt     int64  `json:"countryAt,omitempty"`
}

// TypeChain is the Type of a chain proxy.
const TypeChain = "chain"

// Limits on the number of nodes in a chain proxy.
const (
	MinChainLength = 2
	MaxChainLength = 8
)

// Sources of a node's country (Settings.CountrySource).
const (
	CountryExit   = "exit"   // the IP websites see, found through the node
	CountryServer = "server" // the node server's own IP, looked up locally
)

func (p *Profile) IsChain() bool { return p.Type == TypeChain }

// Server returns "host:port".
func (p *Profile) Server() string {
	host, _ := p.Outbound["server"].(string)
	switch port := p.Outbound["server_port"].(type) {
	case float64:
		return net.JoinHostPort(host, strconv.Itoa(int(port)))
	case int:
		return net.JoinHostPort(host, strconv.Itoa(port))
	}
	return host
}

type Group struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	URL       string            `json:"url"` // subscription URL, empty for a manual group
	UserAgent string            `json:"userAgent,omitempty"`
	UpdatedAt int64             `json:"updatedAt,omitempty"`
	Usage     *SubscriptionInfo `json:"usage,omitempty"`
	Default   bool              `json:"default,omitempty"` // placeholder group created automatically
	// ManualOrder is set when the nodes of a subscription were reordered by
	// hand (dragged or sorted), so that an update can ask before resetting it.
	ManualOrder bool `json:"manualOrder,omitempty"`
	// Clash says which parts of a Clash subscription besides its nodes are
	// taken over on every update; nil until the user was asked.
	Clash *ClashImport `json:"clash,omitempty"`
	// DNS is the subscription's own DNS (订阅专属 DNS); nil = the global one.
	DNS *GroupDNS `json:"dns,omitempty"`
	// SuggestedDNS is the DNS the subscription names (Clash nameserver,
	// sing-box dns.servers), as of its last update; nil when it names none
	// but public ones. Settings → DNS offers it.
	SuggestedDNS *GroupDNS `json:"suggestedDns,omitempty"`
}

// GroupDNS is the DNS of one subscription, used only for its own nodes: other
// subscriptions and the global settings are not affected.
type GroupDNS struct {
	// Remote replaces Settings.RemoteDNS while a node of the subscription is
	// the connected one.
	Remote string `json:"remote,omitempty"`
	// Resolver looks up the server names of its nodes (directly), instead of
	// Settings.DirectDNS.
	Resolver string `json:"resolver,omitempty"`
}

// Empty reports whether d sets nothing.
func (d *GroupDNS) Empty() bool { return d == nil || (d.Remote == "" && d.Resolver == "") }

// ClashImport chooses the parts of a Clash subscription that are converted.
type ClashImport struct {
	Groups bool `json:"groups"` // proxy-groups → policy groups (分组代理)
	Rules  bool `json:"rules"`  // rules → rule sets, one per target
	DNS    bool `json:"dns"`    // nameserver-policy and hosts → DNS overrides
	// SubDNS takes the subscription's own DNS servers (Clash nameserver /
	// proxy-server-nameserver, sing-box dns.servers) as Group.DNS.
	SubDNS bool `json:"subDns"`
}

// SubscriptionInfo is parsed from the "subscription-userinfo" response header.
type SubscriptionInfo struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
	Expire   int64 `json:"expire"`
}

// DNSOverride decides the addresses of some domains: with Server, a DNS
// server of its own answers them (for example the DNS of a provider that
// unlocks streaming sites); with IPs, they have fixed addresses, like a hosts
// file. Server and IPs are not used together.
type DNSOverride struct {
	// Domains are domain suffixes with Server (a domain and its
	// subdomains), full names with IPs.
	Domains  []string `json:"domains"`
	Server   string   `json:"server,omitempty"`   // same forms as RemoteDNS
	ViaProxy bool     `json:"viaProxy,omitempty"` // Server is asked through the proxy
	IPs      []string `json:"ips,omitempty"`
	Disabled bool     `json:"disabled,omitempty"`
	// From is the subscription (group ID) whose Clash DNS settings made
	// this override; an update of the subscription replaces it.
	From int64 `json:"from,omitempty"`
}

type Settings struct {
	Theme       string `json:"theme"` // auto | light | dark
	AccentColor string `json:"accentColor"`
	Language    string `json:"language"`
	// How far the light theme is dimmed and how deep the dark theme goes,
	// 0-100 (see DefaultSettings for the defaults).
	LightLevel int `json:"lightLevel"`
	DarkLevel  int `json:"darkLevel"`
	// ThemeScale is the version of the level scales: 2 since v2.1, whose
	// scales reach further both ways (older levels are converted once).
	ThemeScale   int      `json:"themeScale"`
	RecentColors []string `json:"recentColors,omitempty"` // custom accent colours, newest first

	ListenAddress string `json:"listenAddress"`
	MixedPort     int    `json:"mixedPort"`
	AllowLAN      bool   `json:"allowLan"`
	SystemProxy   bool   `json:"systemProxy"`

	TunEnabled     bool   `json:"tunEnabled"`
	TunStack       string `json:"tunStack"` // system | gvisor | mixed
	TunStrictRoute bool   `json:"tunStrictRoute"`
	TunMTU         int    `json:"tunMtu"`
	// FakeIP answers DNS queries for proxied domains with addresses from a
	// reserved range (TUN mode only); FakeIPExclude lists domain suffixes,
	// one per line, that always get real addresses.
	TunFakeIP     bool   `json:"tunFakeIp"`
	FakeIPExclude string `json:"fakeIpExclude"`
	// TunExclude lists IP ranges (one per line) that stay out of the TUN
	// interface and keep the routes Windows has for them, e.g. Tailscale's
	// 100.64.0.0/10.
	TunExclude string `json:"tunExclude"`

	RouteMode string `json:"routeMode"` // global | rule | direct
	BlockAds  bool   `json:"blockAds"`  // in every mode
	// Rules and Scenes only apply in rule mode. Rules are checked in order;
	// ad blocking and each scene have a place among them (KindAds,
	// KindScene), so that the user decides what is checked first.
	Rules  []Rule  `json:"rules"`
	Scenes []Scene `json:"scenes"`
	// The rule lists of v1.2 and of NextBox for Android (one rule per line);
	// converted into Rules when found, see migrateRules.
	DirectRules   string `json:"directRules,omitempty"`
	ProxyRules    string `json:"proxyRules,omitempty"`
	BlockRules    string `json:"blockRules,omitempty"`
	RuleSetMirror string `json:"ruleSetMirror"`
	// How the routing rules page looks: "path" (the route, steps in order;
	// the default), "cards" (the scene cards and a rule list of v2.0) or
	// "apps" (the service catalog, v2.2). RouteHideOff hides the steps that
	// are switched off (scenes, ad blocking). RouteViewAsked is set once the
	// user has been asked which look to use. Display only.
	RouteView      string `json:"routeView,omitempty"`
	RouteHideOff   bool   `json:"routeHideOff,omitempty"`
	RouteViewAsked bool   `json:"routeViewAsked,omitempty"`
	// ResolveDNS is the DNS server that resolves domain names for the steps
	// matching IP addresses (IP ranges, IP regions, "bypass mainland
	// China"): ResolveRemote (the default, "" too) or ResolveDirect. Known foreign sites
	// are never resolved for this (see config.ForeignSites).
	ResolveDNS string `json:"bypassDns,omitempty"`

	RemoteDNS string `json:"remoteDns"`
	DirectDNS string `json:"directDns"`
	// DNSOverrides (DNS 复写) decide the addresses of some domains before
	// the DNS servers above.
	DNSOverrides []DNSOverride `json:"dnsOverrides,omitempty"`
	EnableIPv6   bool          `json:"enableIpv6"`
	Sniff        bool          `json:"sniff"`

	TestURL         string `json:"testUrl"`
	TestTimeout     int    `json:"testTimeout"` // ms
	TestConcurrency int    `json:"testConcurrency"`
	// SortAfterTest is what happens to the order of a group after a speed
	// test of it: SortAsk ("", the app asks), SortByLatency or SortNone.
	SortAfterTest string `json:"sortAfterTest"`

	SubUserAgent     string `json:"subUserAgent"`
	SubUpdateByProxy bool   `json:"subUpdateByProxy"`
	SubAutoUpdate    bool   `json:"subAutoUpdate"`  // refresh subscriptions in the background
	SubUpdateHours   int    `json:"subUpdateHours"` // ... once they are older than this
	AutoCheckUpdate  bool   `json:"autoCheckUpdate"`

	CountrySource      string `json:"countrySource"`      // CountryExit or CountryServer
	CountryNoticeShown bool   `json:"countryNoticeShown"` // the choice was explained once
	RuleTipShown       bool   `json:"ruleTipShown"`       // rule mode was explained on the home page

	LogLevel       string `json:"logLevel"`
	LogToFile      bool   `json:"logToFile"`    // also write the log to config/logs
	LogFileMaxMB   int    `json:"logFileMaxMb"` // start a new file beyond this size
	LogKeepDays    int    `json:"logKeepDays"`  // delete log files older than this; 0 keeps all
	LogViewLimit   int    `json:"logViewLimit"` // lines kept on the log page
	ConnectOnStart bool   `json:"connectOnStart"`
	AutoStart      bool   `json:"autoStart"`
	// AutoStartConnect connects to the last node when started by Windows.
	AutoStartConnect bool `json:"autoStartConnect"`
	CloseToTray      bool `json:"closeToTray"` // the close button hides the window to the tray
	FindProcess      bool `json:"findProcess"` // resolve the process of each connection (small overhead)
}

const (
	// LocalRuleSetMirror uses the rule-sets bundled in the executable
	// (geosite-cn, geoip-cn, geosite-geolocation-!cn); anything else is fetched
	// from RemoteRuleSetMirror.
	LocalRuleSetMirror  = "local"
	RemoteRuleSetMirror = "https://testingcf.jsdelivr.net/gh/SagerNet/sing-{kind}@rule-set/{name}.srs"
)

// Settings.SortAfterTest values.
const (
	SortAsk       = ""
	SortByLatency = "latency"
	SortNone      = "none"
)

// Settings.ResolveDNS values.
const (
	ResolveDirect = "direct" // the direct DNS: answers as the site is reached from here
	ResolveRemote = "remote" // the remote DNS, through the proxy: never tampered with
)

func DefaultSettings() Settings {
	return Settings{
		Theme:            "auto",
		AccentColor:      "#F472B6",
		Language:         "zh-CN",
		ListenAddress:    "127.0.0.1",
		MixedPort:        2030,
		SystemProxy:      true,
		CloseToTray:      true,
		TunStack:         "mixed",
		TunStrictRoute:   true,
		TunMTU:           9000,
		FakeIPExclude:    DefaultFakeIPExclude,
		RouteMode:        "global", // nothing to set up; rule mode bypasses mainland China
		Scenes:           DefaultScenes(),
		Rules:            []Rule{},
		DarkLevel:        DefaultDarkLevel,
		LightLevel:       DefaultLightLevel,
		ThemeScale:       ThemeScale,
		AutoStartConnect: true,
		RuleSetMirror:    LocalRuleSetMirror,
		ResolveDNS:       ResolveRemote,
		RemoteDNS:        "https://1.1.1.1/dns-query",
		DirectDNS:        "223.5.5.5",
		Sniff:            true,
		TestURL:          "https://www.google.com/generate_204",
		TestTimeout:      5000,
		TestConcurrency:  16,
		SubUserAgent:     "NextBox/2.4.0 (sing-box; ClashMeta compatible)",
		SubUpdateHours:   12,
		CountrySource:    CountryExit,
		LogLevel:         "info",
		LogFileMaxMB:     10,
		LogKeepDays:      7,
		LogViewLimit:     500,
	}
}

// fillDefaults repairs zero values left by older or hand-edited data files.
func (s *Settings) fillDefaults() {
	d := DefaultSettings()
	if s.Theme == "" {
		s.Theme = d.Theme
	}
	if s.AccentColor == "" {
		s.AccentColor = d.AccentColor
	}
	if s.Language == "" {
		s.Language = d.Language
	}
	if s.ListenAddress == "" {
		s.ListenAddress = d.ListenAddress
	}
	if s.MixedPort <= 0 || s.MixedPort > 65535 {
		s.MixedPort = d.MixedPort
	}
	if s.TunStack == "" {
		s.TunStack = d.TunStack
	}
	if s.TunMTU <= 0 {
		s.TunMTU = d.TunMTU
	}
	if s.RouteMode != "rule" && s.RouteMode != "direct" {
		s.RouteMode = "global"
	}
	s.LightLevel = clamp(s.LightLevel, 0, 100)
	s.DarkLevel = clamp(s.DarkLevel, 0, 100)
	s.migrateLevels()
	s.migrateRules()
	s.Scenes = mergeScenes(s.Scenes)
	s.Rules = placeSteps(cleanRules(splitSteam(s.Rules)), s.Scenes)
	if s.RuleSetMirror == "" {
		s.RuleSetMirror = d.RuleSetMirror
	}
	// the remote DNS unless the direct one was chosen (it can answer blocked
	// sites with wrong addresses, see config.ForeignSites)
	if s.ResolveDNS != ResolveDirect {
		s.ResolveDNS = ResolveRemote
	}
	if s.RemoteDNS == "" {
		s.RemoteDNS = d.RemoteDNS
	}
	if s.DirectDNS == "" {
		s.DirectDNS = d.DirectDNS
	}
	if s.TestURL == "" {
		s.TestURL = d.TestURL
	}
	if s.TestTimeout <= 0 {
		s.TestTimeout = d.TestTimeout
	}
	if s.TestConcurrency <= 0 {
		s.TestConcurrency = d.TestConcurrency
	}
	// the default of an older version follows the new one; a custom value stays
	if s.SubUserAgent == "" || (strings.HasPrefix(s.SubUserAgent, "NextBox/") && strings.HasSuffix(s.SubUserAgent, " (sing-box; ClashMeta compatible)")) {
		s.SubUserAgent = d.SubUserAgent
	}
	if s.CountrySource != CountryServer {
		s.CountrySource = CountryExit
	}
	if s.SubUpdateHours <= 0 {
		s.SubUpdateHours = d.SubUpdateHours
	}
	if s.LogLevel == "" {
		s.LogLevel = d.LogLevel
	}
	if s.LogFileMaxMB <= 0 {
		s.LogFileMaxMB = d.LogFileMaxMB
	}
	if s.LogKeepDays < 0 {
		s.LogKeepDays = 0
	}
	if s.LogViewLimit <= 0 {
		s.LogViewLimit = d.LogViewLimit
	}
}
