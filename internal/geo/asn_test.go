package geo

import "testing"

func TestParseASN(t *testing.T) {
	n, err := parseIPSB([]byte(`{"organization":"Cloudflare","isp":"Cloudflare","asn_organization":"Cloudflare, Inc.","asn":13335,"ip":"1.1.1.1"}`))
	if err != nil || n.ASN != 13335 || n.Org != "Cloudflare, Inc." {
		t.Errorf("ip.sb: %+v %v", n, err)
	}
	n, err = parseIPInfo([]byte(`{"ip":"1.1.1.1","org":"AS13335 Cloudflare, Inc."}`))
	if err != nil || n.ASN != 13335 || n.Org != "Cloudflare, Inc." {
		t.Errorf("ipinfo: %+v %v", n, err)
	}
	if _, err = parseIPInfo([]byte(`{"ip":"1.1.1.1","bogon":true}`)); err == nil {
		t.Error("ipinfo without org: no error")
	}
}
