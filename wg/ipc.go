package wg

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/pocketbase/dbx"
	"github.com/shynome/err0"
	"github.com/shynome/err0/try"
	"remoon.net/well/db"
)

func (app *Config) IpcConfig() (_ string, err error) {
	defer err0.Then(&err, nil, nil)

	base := app.base.Load()

	var b = new(bytes.Buffer)

	fmt.Fprintf(b, "private_key=%s\n", hex.EncodeToString(base.key[:]))
	fmt.Fprintf(b, "listen_port=%d\n", base.port)

	q := dbx.HashExp{"disabled": false}
	peers := try.To1(app.FindAllRecords(db.TablePeers, q))
	for _, peer := range peers {
		p := NewPeer(app)
		p.SetProxyRecord(peer)
		ps := try.To1(p.IpcConfig())
		b.WriteString(ps)
	}

	return b.String(), nil
}

func (p *Peer) IpcConfig() (_ string, err error) {
	defer err0.Then(&err, nil, nil)
	var b = new(bytes.Buffer)
	pubkey := p.GetString("pubkey")
	fmt.Fprintf(b, "public_key=%s\n", Base64ToHex(pubkey))

	psk := p.GetString("psk")
	if psk == "" {
		psk = defaultPSK
	}
	fmt.Fprintf(b, "preshared_key=%s\n", Base64ToHex(psk))

	fmt.Fprintf(b, "replace_allowed_ips=true\n")
	ip4 := p.GetString("ipv4")
	ip6 := p.GetString("ipv6")
	allows := []string{ip4, ip6}
	if ip4 != "" {
		ip4in6 := try.To1(ip4in6(ip4))
		allows = append(allows, ip4in6)
	}
	for _, ip := range allows {
		if ip == "" {
			continue
		}
		fmt.Fprintf(b, "allowed_ip=%s\n", ip)
	}

	eps := []string{}
	for _, ep := range []string{p.GetString("endpoint"), p.GetString("endpoint2")} {
		if ep == "" {
			continue
		}
		eps = append(eps, ep)
	}
	if len(eps) > 0 {
		s := try.To1(json.Marshal(eps))
		fmt.Fprintf(b, "endpoint=%s\n", s)
	}

	t := p.GetInt("auto")
	// 只有存在 endpoint 时 auto 配置才有意义
	if len(eps) == 0 {
		t = 0
	}
	fmt.Fprintf(b, "persistent_keepalive_interval=%d\n", t)

	return b.String(), nil
}

const defaultPSK = "0000000000000000000000000000000000000000000000000000000000000000"

func ip4in6(ip4raw string) (string, error) {
	ip4, err := netip.ParsePrefix(ip4raw)
	if err != nil {
		return "", err
	}
	ip4str := ip4.Addr().String()
	ip4in6 := "fdd9:f8f4::" + ip4str + "/128"
	return ip4in6, nil
}
