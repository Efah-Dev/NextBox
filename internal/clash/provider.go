package clash

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"strings"

	"gopkg.in/yaml.v3"

	"nextbox/internal/i18n"
)

// ProviderToSource converts a Clash rule provider (YAML with a payload, or
// plain text, one entry per line; behaviors domain, ipcidr and classical)
// into a sing-box source rule-set, which the core then compiles. The binary
// mrs format is not readable.
func ProviderToSource(body []byte) ([]byte, error) {
	if bytes.HasPrefix(body, []byte("MRS")) {
		return nil, errors.New(i18n.T("不支持 Clash 的 mrs 格式"))
	}
	var lines []string
	var doc struct {
		Payload []string `yaml:"payload"`
	}
	if bytes.Contains(body, []byte("payload:")) && yaml.Unmarshal(body, &doc) == nil {
		lines = doc.Payload
	} else {
		lines = strings.Split(string(body), "\n")
	}
	fields := map[string][]string{}
	add := func(k, v string) {
		if v != "" {
			fields[k] = append(fields[k], v)
		}
	}
	for _, l := range lines {
		l = strings.Trim(strings.TrimSpace(l), `'"`)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "//") {
			continue
		}
		if strings.Contains(l, ",") { // classical: TYPE,value[,option]
			p := strings.Split(l, ",")
			v := strings.TrimSpace(p[1])
			switch strings.ToUpper(strings.TrimSpace(p[0])) {
			case "DOMAIN":
				add("domain", v)
			case "DOMAIN-SUFFIX":
				add("domain_suffix", v)
			case "DOMAIN-KEYWORD":
				add("domain_keyword", v)
			case "DOMAIN-REGEX":
				add("domain_regex", v)
			case "IP-CIDR", "IP-CIDR6":
				add("ip_cidr", v)
			case "PROCESS-NAME":
				add("process_name", v)
			}
			continue
		}
		if _, _, err := net.ParseCIDR(l); err == nil {
			add("ip_cidr", l)
			continue
		}
		if ip := net.ParseIP(l); ip != nil {
			add("ip_cidr", ip.String())
			continue
		}
		switch {
		case strings.HasPrefix(l, "+."), strings.HasPrefix(l, "*."):
			add("domain_suffix", l[2:])
		case strings.HasPrefix(l, "."):
			add("domain_suffix", l[1:])
		default:
			add("domain", l)
		}
	}
	if len(fields) == 0 {
		return nil, errors.New(i18n.T("规则源里没有能识别的规则"))
	}
	// one rule per field: in one rule, a process name would have to match
	// together with the domains instead of besides them
	var rules []map[string]any
	for _, k := range []string{"domain", "domain_suffix", "domain_keyword", "domain_regex", "ip_cidr", "process_name"} {
		if v := fields[k]; len(v) > 0 {
			rules = append(rules, map[string]any{k: v})
		}
	}
	return json.Marshal(map[string]any{"version": 2, "rules": rules})
}
