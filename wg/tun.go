//go:build !windows

package wg

import "golang.zx2c4.com/wireguard/tun"

func tunFromFD(fd int) (tun.Device, error) {
	tdev, _, err := tun.CreateUnmonitoredTUNFromFD(fd)
	return tdev, err
}
