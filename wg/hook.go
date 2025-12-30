package wg

import (
	"bytes"
	"fmt"
	"math/big"
	"net/netip"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/shynome/err0"
	"github.com/shynome/err0/try"
	"go4.org/netipx"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"remoon.net/well/db"
)

func BindHook(se *core.ServeEvent) error {
	// 检查数据是否合法
	preUpdateRequest(se.App, db.TablePeers, func(e *core.RecordRequestEvent) error {
		r := e.Record
		pubkey := r.GetString("pubkey")
		if _, err := wgtypes.ParseKey(pubkey); err != nil {
			return apis.NewBadRequestError("pubkey 解析失败", err)
		}
		if psk := r.GetString("psk"); psk != "" {
			if _, err := wgtypes.ParseKey(psk); err != nil {
				return apis.NewBadRequestError("psk 解析失败", err)
			}
		}
		return preUpdatePeer(e)
	})
	// 操作
	preUpdateRequest(se.App, db.TablePeers, func(e *core.RecordRequestEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		p := NewPeer(e.App)
		p.SetProxyRecord(e.Record)
		ipcConf, err := p.IpcConfig()
		if err != nil {
			return apis.NewInternalServerError("peer get ipc config failed", err)
		}
		pubkey := Base64ToHex(p.GetString("pubkey"))
		devLocker.Lock()
		defer devLocker.Unlock()
		if dev == nil {
			return nil
		}
		rmConf := fmt.Sprintf("public_key=%s\nremove=true\n", pubkey)
		if err := dev.IpcSet(rmConf); err != nil {
			return apis.NewInternalServerError("WireGuard rm peer failed", err)
		}
		if err := dev.IpcSet(ipcConf); err != nil {
			return apis.NewInternalServerError("WireGuard set ipc config failed", err)
		}
		return nil
	})
	se.App.OnRecordDeleteRequest(db.TablePeers).BindFunc(func(e *core.RecordRequestEvent) error {
		devLocker.Lock()
		defer devLocker.Unlock()
		if dev == nil {
			return e.Next()
		}
		pubkey := Base64ToHex(e.Record.GetString("pubkey"))
		var b = new(bytes.Buffer)
		fmt.Fprintf(b, "public_key=%s\n", pubkey)
		fmt.Fprintf(b, "remove=true\n")
		if err := dev.IpcSet(b.String()); err != nil {
			msg := fmt.Sprintf("删除节点出错: %e", err)
			return apis.NewInternalServerError(msg, err)
		}
		return e.Next()
	})

	return se.Next()
}

func preUpdatePeer(e *core.RecordRequestEvent) (err error) {
	defer err0.Then(&err, nil, nil)

	r := e.Record

	routes := getRoutes()
	var (
		pf6 = try.To1(netip.ParsePrefix(routes[0]))
		pf4 = try.To1(netip.ParsePrefix(routes[1]))
	)

	if ip6 := r.GetString("ipv6"); ip6 != "" {
		ip6 := try.To1(netip.ParsePrefix(ip6)).Addr()
		if !pf6.Contains(ip6) {
			msg := fmt.Sprintf("ipv6 地址不在 %s 范围内", pf6.String())
			return apis.NewBadRequestError(msg, nil)
		}
	}

	ipf4Str := r.GetString("ipv4")
	switch ipf4Str {
	case "":
		return e.Next()
	case "auto":
		peers := try.To1(e.App.FindRecordsByFilter(db.TablePeers, "", "-ip_num", 1, 0))
		addr := pf4.Addr()
		bits := 32
		if len(peers) == 0 {
			ipf4Str = netip.PrefixFrom(addr.Next(), bits).String()
		} else {
			n := int64(peers[0].GetInt("ip_num"))
			n += 1
			z := new(big.Int)
			z.SetBytes(addr.AsSlice())
			z = z.Add(z, big.NewInt(n))
			ip, ok := netip.AddrFromSlice(z.Bytes())
			if !ok {
				return apis.NewInternalServerError("不应当如此", nil)
			}
			ipf4Str = netip.PrefixFrom(ip, bits).String()
		}
	}
	r.Set("ipv4", ipf4Str)

	// 计算下一个 auto ip的起点
	ipf4, err := netip.ParsePrefix(ipf4Str)
	if err != nil {
		return apis.NewBadRequestError("ip 解析失败", err)
	}
	ip := netipx.PrefixLastIP(ipf4)
	lipStr := ip.String()
	_ = lipStr
	if !pf4.Contains(ip) {
		msg := fmt.Sprintf("ip(%s) 地址不在 route(%s) 的范围内", ipf4Str, pf4.String())
		return apis.NewBadRequestError(msg, err)
	}
	ip1 := new(big.Int)
	ip1.SetBytes(pf4.Addr().AsSlice())
	ip2 := new(big.Int)
	ip2.SetBytes(ip.AsSlice())
	dis := new(big.Int)
	dis = dis.Sub(ip2, ip1)
	n := dis.Int64()
	if n == 0 {
		return apis.NewBadRequestError("不可和本机地址一样", nil)
	}
	r.Set("ip_num", n)
	return e.Next()
}

func preUpdateRequest(app core.App, table string, fn func(e *core.RecordRequestEvent) error) {
	app.OnRecordCreateRequest(table).BindFunc(fn)
	app.OnRecordUpdateRequest(table).BindFunc(fn)
}
