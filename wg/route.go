package wg

import (
	"golang.zx2c4.com/wireguard/tun"
)

var RouteUp func(tun tun.Device, routes []string) error
