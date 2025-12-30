package wg

import (
	"net"
	"net/netip"

	"github.com/shynome/err0"
	"github.com/shynome/err0/try"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

func init() {
	RouteUp = windowsRouteUp
}

func windowsRouteUp(tun tun.Device, routes []string) (err error) {
	defer err0.Then(&err, nil, nil)
	name := try.To1(tun.Name())
	iface := try.To1(net.InterfaceByName(name))
	luid := try.To1(winipcfg.LUIDFromIndex(uint32(iface.Index)))
	addrs := []netip.Prefix{}
	for _, route := range routes {
		pf := try.To1(netip.ParsePrefix(route))
		addrs = append(addrs, pf)
	}
	return luid.AddIPAddresses(addrs)
}
