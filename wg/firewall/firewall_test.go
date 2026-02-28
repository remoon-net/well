package firewall

import (
	"net/netip"
	"testing"
	"time"

	"remoon.net/well/wg/conntrack"
)

func TestUDPTracker(t *testing.T) {
	logger := NoopLogger{}
	fl := NoopFlowLogger{}
	tracker := conntrack.NewUDPTracker(2*time.Second, logger, fl)
	defer tracker.Close()
	ip1 := netip.MustParseAddr("127.0.0.1")
	ip2 := netip.MustParseAddr("127.0.0.2")
	tracker.TrackOutbound(ip1, ip2, 80, 80, 0)
	valid := tracker.IsValidInbound(ip2, ip1, 80, 80, 0)
	t.Log(valid)
	time.Sleep(1 * time.Second)
	valid2 := tracker.IsValidInbound(ip2, ip1, 80, 80, 0)
	t.Log(valid2)
	time.Sleep(1 * time.Second)
	valid3 := tracker.IsValidInbound(ip2, ip1, 80, 80, 0)
	t.Log(valid3)
	time.Sleep(3 * time.Second)
	valid4 := tracker.IsValidInbound(ip2, ip1, 80, 80, 0)
	t.Log(valid4)
}
