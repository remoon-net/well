package wg

import (
	"errors"

	"github.com/shynome/err0"
	"github.com/shynome/wgortc/device/vtun"
	"golang.zx2c4.com/wireguard/tun"
)

var ErrNotGVisorStack = errors.New("the tun device can't get a gVisor stack")

func vtunRouteUp(tun tun.Device, routes []string) (err error) {
	defer err0.Then(&err, nil, nil)
	tdev, ok := tun.(vtun.GetStack)
	if !ok {
		return ErrNotGVisorStack
	}
	return vtun.RouteUp(tdev, routes)
}

func init() {
	_ = vtunRouteUp // 目前不会用到这个
}
