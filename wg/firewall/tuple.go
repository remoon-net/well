package firewall

import (
	"net/netip"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type Tuple struct {
	Proto   tcpip.TransportProtocolNumber
	Src     netip.Addr
	SrcPort uint16
	Dst     netip.Addr
	DstPort uint16
}

// 返回 nil 时表示不需要进行过滤
func getTuple(buf []byte) *Tuple {
	var packet header.Network
	var (
		src tcpip.Address
		dst tcpip.Address
	)
	v := header.IPVersion(buf)
	switch v {
	case 4:
		pkt := header.IPv4(buf)
		src, dst = pkt.SourceAddress(), pkt.DestinationAddress()
		packet = pkt
	case 6:
		pkt := header.IPv6(buf)
		src, dst = pkt.SourceAddress(), pkt.DestinationAddress()
		packet = pkt
	default:
		return nil
	}
	payload := packet.Payload()
	var (
		sport uint16
		dport uint16
	)
	p := packet.TransportProtocol()
	switch p {
	case header.TCPProtocolNumber:
		tcp := header.TCP(payload)
		sport, dport = tcp.SourcePort(), tcp.DestinationPort()
		// For TCP, we want to allow *outgoing* connections,
		// which means we want to allow return packets on those
		// connections. To make this restriction work, we need to
		// allow non-SYN packets (continuation of an existing session)
		// to arrive. This should be okay since a new incoming session
		// can't be initiated without first sending a SYN.
		// It happens to also be much faster.
		// TODO(apenwarr): Skip the rest of decoding in this path?
		//
		// copy from tailscale tailscale/wgengine/filter
		if !IsTCPSyn(tcp) {
			// log.Println("返回包:", src, sport, dst, dport)
			return nil
		}
	case header.UDPProtocolNumber:
		udp := header.UDP(payload)
		sport, dport = udp.SourcePort(), udp.DestinationPort()
	default:
		return nil
	}

	src1, _ := netip.AddrFromSlice(src.AsSlice())
	dst1, _ := netip.AddrFromSlice(dst.AsSlice())

	return &Tuple{
		Proto:   p,
		Src:     src1,
		SrcPort: sport,
		Dst:     dst1,
		DstPort: dport,
	}
}

const TCPSynAck = header.TCPFlagSyn | header.TCPFlagAck

func IsTCPSyn(tcp header.TCP) bool {
	return (tcp.Flags() & TCPSynAck) == header.TCPFlagSyn
}
