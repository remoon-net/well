package wg

import (
	"bytes"
	"fmt"
	"math/big"
	"net/netip"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/shynome/err0"
	"github.com/shynome/err0/try"
	"go4.org/netipx"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"remoon.net/well/db"
)

func InitHook(app core.App) error {
	// 检查数据是否合法
	preUpdateRequest(app, db.TablePeers, 0, func(e *core.RecordRequestEvent) error {
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
	app.OnRecordsListRequest(db.TablePeers).BindFunc(func(e *core.RecordsListRequestEvent) error {
		for _, p := range e.Records {
			if ip4 := p.GetString("ipv4"); ip4 != "" {
				ip6, err := ip4in6(ip4)
				if err != nil {
					return err
				}
				e := p.Expand()
				e["ip4in6"] = ip6
				p.SetExpand(e)
			}
		}
		return e.Next()
	})
	// 操作
	preUpdateRequest(app, db.TablePeers, lastOrder, func(e *core.RecordRequestEvent) error {
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
		dev := wgBind.GetDevice()
		if dev == nil {
			return nil
		}
		rmConf := fmt.Sprintf("public_key=%s\nremove=true\n", pubkey)
		if err := dev.IpcSet(rmConf); err != nil {
			return apis.NewInternalServerError("WireGuard rm peer failed", err)
		}
		if p.GetBool("disabled") { // 如果已经被禁用, 就不再添加
			return nil
		}
		if err := dev.IpcSet(ipcConf); err != nil {
			return apis.NewInternalServerError("WireGuard set ipc config failed", err)
		}
		return nil
	})
	app.OnRecordDeleteRequest(db.TablePeers).BindFunc(func(e *core.RecordRequestEvent) error {
		devLocker.Lock()
		defer devLocker.Unlock()
		dev := wgBind.GetDevice()
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

	return nil
}

func preUpdatePeer(e *core.RecordRequestEvent) (err error) {
	defer err0.Then(&err, nil, nil)

	r := e.Record

	routes := GetRoutes()

	ip6auto := func() error {
		pf6 := try.To1(netip.ParsePrefix(routes[0]))
		pf6auto := try.To1(netip.ParsePrefix("2001:00f0::1/32"))
		ipf6Str := r.GetString("ipv6")
		switch ipf6Str {
		case "":
			return nil
		case "auto":
			pf6 := pf6
			if !pf6auto.Contains(pf6.Addr()) {
				pf6 = pf6auto
			}
			peers := try.To1(e.App.FindRecordsByFilter(db.TablePeers, "", "-ip6_num", 1, 0))
			addr := pf6.Addr()
			bits := 128
			if len(peers) == 0 {
				ipf6Str = netip.PrefixFrom(addr.Next(), bits).String()
			} else {
				n := int64(peers[0].GetInt("ip6_num"))
				n += 1
				z := new(big.Int)
				z.SetBytes(addr.AsSlice())
				z = z.Add(z, big.NewInt(n))
				ip, ok := netip.AddrFromSlice(z.Bytes())
				if !ok {
					return apis.NewInternalServerError("不应当如此", nil)
				}
				ipf6Str = netip.PrefixFrom(ip, bits).String()
			}
		}
		r.Set("ipv6", ipf6Str)

		// 计算下一个 auto ip的起点
		ipf6, err := netip.ParsePrefix(ipf6Str)
		if err != nil {
			return apis.NewBadRequestError("ip 解析失败", err)
		}
		ip := netipx.PrefixLastIP(ipf6)
		lipStr := ip.String()
		_ = lipStr
		if !pf6.Contains(ip) {
			msg := fmt.Sprintf("ip(%s) 地址不在 route(%s) 的范围内", ipf6Str, pf6.String())
			return apis.NewBadRequestError(msg, err)
		}

		// 不是 pf6auto 地址范围内的跳过, 不用计算 ip6_num
		if !pf6auto.Contains(ipf6.Addr()) {
			return nil
		}

		ip1 := new(big.Int)
		ip1.SetBytes(pf6auto.Addr().AsSlice())
		ip2 := new(big.Int)
		ip2.SetBytes(ip.AsSlice())
		dis := new(big.Int)
		dis = dis.Sub(ip2, ip1)
		n := dis.Int64()
		if n == 0 {
			return apis.NewBadRequestError("不可和本机地址一样", nil)
		}
		r.Set("ip6_num", n)
		return nil
	}
	if err := ip6auto(); err != nil {
		return err
	}

	ip4auto := func() error {
		pf4 := try.To1(netip.ParsePrefix(routes[1]))

		ipf4Str := r.GetString("ipv4")
		switch ipf4Str {
		case "":
			return nil
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
		return nil
	}
	if err := ip4auto(); err != nil {
		return err
	}

	return e.Next()
}

const lastOrder = 99999

func preUpdateRequest(app core.App, table string, order int, fn func(e *core.RecordRequestEvent) error) {
	h := &hook.Handler[*core.RecordRequestEvent]{
		Priority: order,
		Func:     fn,
	}
	app.OnRecordCreateRequest(table).Bind(h)
	app.OnRecordUpdateRequest(table).Bind(h)
}
