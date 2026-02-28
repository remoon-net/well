package firewall

import (
	"github.com/pocketbase/pocketbase/tools/store"
	"golang.zx2c4.com/wireguard/tun"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"remoon.net/well/wg/conntrack"
)

type firewallTun struct {
	tun.Device
	udpTracker *conntrack.UDPTracker
	allows     *ProtoAllows
}

var _ tun.Device = (*firewallTun)(nil)

func NewTun(dev tun.Device, allows *ProtoAllows) *firewallTun {
	logger := NoopLogger{}
	fl := NoopFlowLogger{}

	udp := conntrack.NewUDPTracker(conntrack.DefaultUDPTimeout, logger, fl)
	return &firewallTun{
		Device:     dev,
		udpTracker: udp,
		allows:     allows,
	}
}

func (ft *firewallTun) Close() error {
	ft.udpTracker.Close()
	return ft.Device.Close()
}

func (ft *firewallTun) Read(bufs [][]byte, sizes []int, offset int) (n int, err error) {
	n, err = ft.Device.Read(bufs, sizes, offset)
	if err != nil {
		return n, err
	}
	for i := 0; i < n; i++ {
		pkt := bufs[i][offset : offset+sizes[i]]
		t := getTuple(pkt)
		if t != nil {
			switch t.Proto {
			case header.UDPProtocolNumber:
				ft.udpTracker.TrackOutbound(
					t.Src, t.Dst,
					t.SrcPort, t.DstPort,
					sizes[i],
				)
			}
		}
	}
	return n, nil
}

func (ft *firewallTun) Write(bufs [][]byte, offset int) (int, error) {
	filtered := bufs[:0]
	for _, buf := range bufs {
		pkt := buf[offset:]
		t := getTuple(pkt)
		if !ft.allow(t) {
			continue
		}
		filtered = append(filtered, buf)
	}
	return ft.Device.Write(filtered, offset)
}

func (ft *firewallTun) allow(t *Tuple) bool {
	if t == nil {
		return true
	}
	var allows *store.Store[uint16, empty]
	switch t.Proto {
	case header.TCPProtocolNumber:
		allows = ft.allows.TCP
	case header.UDPProtocolNumber:
		allows = ft.allows.UDP
		valid := ft.udpTracker.IsValidInbound(
			t.Src, t.Dst,
			t.SrcPort, t.DstPort,
			0,
		)
		if valid {
			return true
		}
	default:
		return true
	}
	if !allows.Has(t.DstPort) {
		// log.Println("被拒绝:", t.Src, t.SrcPort, t.Dst, t.DstPort)
		return false
	}
	return true
}

type empty = struct{}
