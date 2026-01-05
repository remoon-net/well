package wg

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sync/atomic"

	"github.com/pocketbase/pocketbase/core"
	"github.com/shynome/err0"
	"github.com/shynome/err0/try"
	"github.com/shynome/wgortc/bind"
	"github.com/shynome/wgortc/device/pubkey"
	"github.com/shynome/wgortc/nat"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"remoon.net/well/db"
)

type Config struct {
	core.App
	base atomic.Pointer[configBase]
}

var _ bind.Config = (*Config)(nil)

type configBase struct {
	key  device.NoisePrivateKey
	dst  [2]netip.Addr // [0]v6 [1]v4
	port uint16
}

func (app *Config) GetPeer(initiator []byte, endpoint string) (peer bind.Peer) {
	var err error
	logger := app.Logger()
	defer err0.Then(&err, nil, func() {
		logger.Error("get peer failed", "error", err)
		peer = nil
	})
	var p *Peer = NewPeer(app)
	base := app.base.Load()
	switch {
	case endpoint != "":
		eps := []string{}
		try.To(json.Unmarshal([]byte(endpoint), &eps)) // Config 的都是
		if len(eps) < 1 {
			err0.Throw(fmt.Errorf("不应当出现无法获取 endpoints 的情况. endpoint: %s", endpoint))
		}
		record := try.To1(app.FindFirstRecordByData(db.TablePeers, "endpoint", eps[0]))
		p.SetProxyRecord(record)
		p.outbound = true
	case len(initiator) != 0:
		pubkey := try.To1(pubkey.Unpack(base.key, initiator))
		pubkeyStr := wgtypes.Key(pubkey).String()
		record, err := app.FindFirstRecordByData(db.TablePeers, "pubkey", pubkeyStr)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				peers := try.To1(app.FindCachedCollectionByNameOrId(db.TablePeers))
				record = core.NewRecord(peers)
				record.Set("pubkey", pubkeyStr)
				record.Set("disabled", true)
				try.To(app.Save(record))
			}
			return nil
		}
		p.SetProxyRecord(record)
	}

	if p.GetBool("disabled") {
		return nil
	}

	n := nat.New()
	dst6, dst4 := base.dst[0], base.dst[1]
	ip6 := p.GetString("ipv6")
	ip4 := p.GetString("ipv4")
	if ip4 != "" {
		src := try.To1(netip.ParsePrefix(ip4)).Addr()
		n.SetNAT4(src, dst4)
		if ip6 == "" {
			ip6 = try.To1(ip4in6(ip4))
		}
	}
	if ip6 != "" {
		src := try.To1(netip.ParsePrefix(ip6)).Addr()
		n.SetNAT6(src, dst6)
	}
	p.INAT = n

	return p
}
