//go:build !android

package wg

import (
	"net/netip"
	"os"
	"os/exec"

	"github.com/shynome/err0"
	"github.com/shynome/err0/try"
	"golang.zx2c4.com/wireguard/tun"
)

func init() {
	RouteUp = linuxRouteUp
}

func linuxRouteUp(tun tun.Device, routes []string) (err error) {
	defer err0.Then(&err, nil, nil)
	name := try.To1(tun.Name())
	for _, route := range routes {
		pf := try.To1(netip.ParsePrefix(route))
		cmd := exec.Command("ip", "addr", "add", pf.String(), "dev", name)
		cmd.Stderr = os.Stderr
		try.To(cmd.Run())
	}
	up := exec.Command("ip", "link", "set", name, "up")
	up.Stderr = os.Stderr
	return up.Run()
}
