package wg

import (
	"fmt"

	"golang.zx2c4.com/wireguard/tun"
)

func tunFromFD(fd int) (tun.Device, error) {
	return nil, fmt.Errorf(`windows don't support tun from fd %d`, fd)
}
