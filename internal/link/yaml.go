package link

import (
	"regexp"

	"gopkg.in/yaml.v3"
)

// UnmarshalYAML decodes YAML like yaml.Unmarshal, except that a key repeated
// in one mapping does not fail the whole document: the last one wins, as in
// Clash. Generated Clash configs often repeat keys such as "udp".
func UnmarshalYAML(data []byte, out any) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	dropDuplicateKeys(&doc, map[*yaml.Node]bool{})
	return doc.Decode(out)
}

func dropDuplicateKeys(n *yaml.Node, seen map[*yaml.Node]bool) {
	if n == nil || seen[n] {
		return
	}
	seen[n] = true
	if n.Kind == yaml.MappingNode {
		last := map[string]int{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			if k := n.Content[i]; k.Kind == yaml.ScalarNode && k.Value != "<<" {
				last[k.Value] = i
			}
		}
		if len(last)*2 < len(n.Content) {
			kept := make([]*yaml.Node, 0, len(n.Content))
			for i := 0; i+1 < len(n.Content); i += 2 {
				k := n.Content[i]
				if k.Kind == yaml.ScalarNode && k.Value != "<<" && last[k.Value] != i {
					continue
				}
				kept = append(kept, k, n.Content[i+1])
			}
			n.Content = kept
		}
	}
	for _, c := range n.Content {
		dropDuplicateKeys(c, seen)
	}
	dropDuplicateKeys(n.Alias, seen)
}

// clashTopKey matches a top-level key only a Clash config has.
var clashTopKey = regexp.MustCompile(`(?m)^(proxies|Proxy|proxy-providers|proxy-groups|Proxy Group|mixed-port|socks-port|port|redir-port|allow-lan|external-controller|rules|Rule)\s*:`)

// looksClash tells a Clash / Clash.Meta config from a list of share links.
func looksClash(text string) bool {
	return clashTopKey.MatchString(text)
}
