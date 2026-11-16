package geo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// Network is the autonomous system (the network operator) an IP belongs to.
type Network struct {
	ASN int    `json:"asn"`
	Org string `json:"org"`
}

// ASN services, tried in this order. Both answer without an account; the
// request goes through the node, so they only learn the exit IP.
var asnServices = []struct {
	url   string // %s = the IP
	parse func([]byte) (Network, error)
}{
	{"https://api.ip.sb/geoip/%s", parseIPSB},
	{"https://ipinfo.io/%s/json", parseIPInfo},
}

// LookupASN asks which network ip belongs to.
func LookupASN(ctx context.Context, client *http.Client, ip string) (Network, error) {
	if net.ParseIP(ip) == nil {
		return Network{}, errors.New("not an IP")
	}
	var errs []error
	for _, s := range asnServices {
		n, err := askASN(ctx, client, fmt.Sprintf(s.url, ip), s.parse)
		if err == nil && n.ASN > 0 {
			return n, nil
		}
		if err == nil {
			err = errors.New("no ASN in the answer")
		}
		errs = append(errs, err)
	}
	return Network{}, errors.Join(errs...)
}

func askASN(ctx context.Context, client *http.Client, url string, parse func([]byte) (Network, error)) (Network, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Network{}, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return Network{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Network{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<10))
	if err != nil {
		return Network{}, err
	}
	return parse(body)
}

// {"asn":13335,"asn_organization":"Cloudflare, Inc.",...}
func parseIPSB(b []byte) (Network, error) {
	var v struct {
		ASN int    `json:"asn"`
		Org string `json:"asn_organization"`
		ISP string `json:"isp"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return Network{}, err
	}
	org := v.Org
	if org == "" {
		org = v.ISP
	}
	return Network{ASN: v.ASN, Org: strings.TrimSpace(org)}, nil
}

// {"org":"AS13335 Cloudflare, Inc.",...}
func parseIPInfo(b []byte) (Network, error) {
	var v struct {
		Org string `json:"org"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return Network{}, err
	}
	as, org, _ := strings.Cut(strings.TrimSpace(v.Org), " ")
	n, err := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(as), "AS"))
	if err != nil || !strings.HasPrefix(strings.ToUpper(as), "AS") {
		return Network{}, errors.New("no ASN in the answer")
	}
	return Network{ASN: n, Org: strings.TrimSpace(org)}, nil
}
