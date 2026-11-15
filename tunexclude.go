package main

// TUN 排除网段: IP ranges that stay out of the TUN interface.

import (
	"net"
	"net/netip"
	"sort"
	"strings"

	"nextbox/internal/config"
)

// CheckCIDRs returns the line numbers (from 1) of a TUN exclusion list that
// are neither an IP range nor an address.
func (a *App) CheckCIDRs(text string) []int {
	_, bad := config.ParseCIDRs(text)
	if bad == nil {
		bad = []int{}
	}
	return bad
}

// Adapter is a network interface of this computer with the networks it is
// on, for 从网卡添加.
type Adapter struct {
	Name     string   `json:"name"`
	Networks []string `json:"networks"`
}

// ListAdapters lists the interfaces that are up and have an address,
// except loopback and NextBox's own TUN interface.
func (a *App) ListAdapters() []Adapter {
	ifs, err := net.Interfaces()
	if err != nil {
		return []Adapter{}
	}
	out := []Adapter{}
	for _, it := range ifs {
		if it.Flags&net.FlagUp == 0 || it.Flags&net.FlagLoopback != 0 || strings.EqualFold(it.Name, "NextBox") {
			continue
		}
		addrs, _ := it.Addrs()
		var nets []string
		for _, ad := range addrs {
			ipn, ok := ad.(*net.IPNet)
			if !ok {
				continue
			}
			ip, ok := netip.AddrFromSlice(ipn.IP)
			if !ok {
				continue
			}
			ip = ip.Unmap()
			if ip.IsLinkLocalUnicast() || ip.IsLoopback() {
				continue
			}
			ones, _ := ipn.Mask.Size()
			nets = append(nets, netip.PrefixFrom(ip, ones).Masked().String())
		}
		if len(nets) > 0 {
			sort.Strings(nets)
			out = append(out, Adapter{Name: it.Name, Networks: nets})
		}
	}
	return out
}
