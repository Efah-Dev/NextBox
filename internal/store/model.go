package store

import (
	"net"
	"strconv"
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
	TestedAt     int64  `json:"testedAt,omitempty"`
	Upload       int64  `json:"upload"`
	Download     int64  `json:"download"`
}

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
}

// SubscriptionInfo is parsed from the "subscription-userinfo" response header.
type SubscriptionInfo struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
	Expire   int64 `json:"expire"`
}

type Settings struct {
	Theme       string `json:"theme"` // auto | light | dark
	AccentColor string `json:"accentColor"`
	Language    string `json:"language"`

	ListenAddress string `json:"listenAddress"`
	MixedPort     int    `json:"mixedPort"`
	AllowLAN      bool   `json:"allowLan"`
	SystemProxy   bool   `json:"systemProxy"`

	TunEnabled     bool   `json:"tunEnabled"`
	TunStack       string `json:"tunStack"` // system | gvisor | mixed
	TunStrictRoute bool   `json:"tunStrictRoute"`
	TunMTU         int    `json:"tunMtu"`

	RouteMode     string `json:"routeMode"` // rule | global | direct
	BlockAds      bool   `json:"blockAds"`
	DirectRules   string `json:"directRules"` // one rule per line, see config.parseRuleLines
	ProxyRules    string `json:"proxyRules"`
	BlockRules    string `json:"blockRules"`
	RuleSetMirror string `json:"ruleSetMirror"`

	RemoteDNS  string `json:"remoteDns"`
	DirectDNS  string `json:"directDns"`
	EnableIPv6 bool   `json:"enableIpv6"`
	Sniff      bool   `json:"sniff"`

	TestURL         string `json:"testUrl"`
	TestTimeout     int    `json:"testTimeout"` // ms
	TestConcurrency int    `json:"testConcurrency"`

	SubUserAgent     string `json:"subUserAgent"`
	SubUpdateByProxy bool   `json:"subUpdateByProxy"`

	LogLevel       string `json:"logLevel"`
	LogToFile      bool   `json:"logToFile"`    // also write the log to config/logs
	LogFileMaxMB   int    `json:"logFileMaxMb"` // start a new file beyond this size
	LogKeepDays    int    `json:"logKeepDays"`  // delete log files older than this; 0 keeps all
	LogViewLimit   int    `json:"logViewLimit"` // lines kept on the log page
	ConnectOnStart bool   `json:"connectOnStart"`
	AutoStart      bool   `json:"autoStart"`
	CloseToTray    bool   `json:"closeToTray"` // the close button hides the window to the tray
	FindProcess    bool   `json:"findProcess"` // resolve the process of each connection (small overhead)
}

func DefaultSettings() Settings {
	return Settings{
		Theme:           "auto",
		AccentColor:     "#F472B6",
		Language:        "zh-CN",
		ListenAddress:   "127.0.0.1",
		MixedPort:       2030,
		SystemProxy:     true,
		CloseToTray:     true,
		TunStack:        "mixed",
		TunStrictRoute:  true,
		TunMTU:          9000,
		RouteMode:       "rule",
		RuleSetMirror:   "https://testingcf.jsdelivr.net/gh/SagerNet/sing-{kind}@rule-set/{name}.srs",
		RemoteDNS:       "https://1.1.1.1/dns-query",
		DirectDNS:       "223.5.5.5",
		Sniff:           true,
		TestURL:         "https://www.google.com/generate_204",
		TestTimeout:     5000,
		TestConcurrency: 16,
		SubUserAgent:    "NextBox/1.0.0 (sing-box; ClashMeta compatible)",
		LogLevel:        "info",
		LogFileMaxMB:    10,
		LogKeepDays:     7,
		LogViewLimit:    500,
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
	if s.RouteMode == "" {
		s.RouteMode = d.RouteMode
	}
	if s.RuleSetMirror == "" {
		s.RuleSetMirror = d.RuleSetMirror
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
	if s.SubUserAgent == "" {
		s.SubUserAgent = d.SubUserAgent
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
