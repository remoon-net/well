package wg

import (
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/shynome/err0"
	"github.com/shynome/err0/try"
	"github.com/shynome/wgortc/bind"
	"github.com/shynome/wgortc/nat"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"remoon.net/well/db"
)

type Peer struct {
	nat.INAT
	core.BaseRecordProxy
	app      core.App
	outbound bool
}

type ICE struct {
	URLs       string `json:"urls"`
	Username   string `json:"username"`
	Credential string `json:"credential"`
}

var _ bind.Peer = (*Peer)(nil)
var _ bind.PeerMode = (*Peer)(nil)
var _ nat.INAT = (*Peer)(nil)
var _ bind.PeerPubkey = (*Peer)(nil)
var _ bind.PeerHandshakeHook = (*Peer)(nil)

func (p *Peer) GetID() string {
	return p.GetID()
}

func (p *Peer) GetPubkey() device.NoisePublicKey {
	pubkey := decodeBase64(p.GetString("pubkey"))
	return device.NoisePublicKey(pubkey)
}

func (p *Peer) GetPeerInit() (c webrtc.Configuration) {
	var err error
	logger := p.app.Logger().With("peer", p)
	defer err0.Then(&err, nil, func() {
		logger.Error("GetPeerInit failed", "error", err)
	})
	c = webrtc.Configuration{
		// ICETransportPolicy: webrtc.ICETransportPolicyAll,
	}
	// if relayOnly := p.GetBool("relay_only"); relayOnly {
	// 	c.ICETransportPolicy = webrtc.ICETransportPolicyRelay
	// }
	var q dbx.Expression = dbx.HashExp{"default": true}
	if ids := p.GetStringSlice("ices"); len(ids) > 0 {
		q = dbx.In("id", ids)
	}
	ices := try.To1(p.app.FindAllRecords(db.TableICEs, q))
	for _, ice := range ices {
		srv := webrtc.ICEServer{
			URLs: []string{
				ice.GetString("urls"),
			},
		}
		if uname := ice.GetString("username"); uname != "" {
			srv.Username = uname
		}
		if pass := ice.GetString("credential"); pass != "" {
			srv.Credential = pass
		}
		c.ICEServers = append(c.ICEServers, srv)
	}
	return c
}

func (p *Peer) TransportMode() bind.TransportMode {
	mm := p.GetStringSlice("transport_mode")

	mode := bind.TransportMode(0)
	for _, m := range mm {
		switch m {
		case db.TransportModeNoWS:
			mode |= bind.WSTransportDisabled
		case db.TransportModeNoWebRTC:
			mode |= bind.WebRTCTransportDisabled
		case db.TransportModeWSRedirect:
			mode |= bind.WSRedirectEnabled
		}
	}

	return mode
}

func (p *Peer) HandshakeInitiationHook(initiator *bind.HandshakeInitiation) {}
func (p *Peer) HandshakeResponseHook(hresp *bind.HandshakeResponse)         {}
func (p *Peer) HandshakedHook(ep conn.Endpoint) {
	handshaked, _ := types.ParseDateTime(time.Now())
	q := dbx.HashExp{"id": p.Id}
	params := dbx.Params{"handshaked": handshaked}
	if _, err := p.app.DB().Update(db.TablePeers, params, q).Execute(); err != nil {
		p.app.Logger().Error("更新 handshaked 出错", "peer", p, "error", err)
	}
}
