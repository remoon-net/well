package wg

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"runtime"
	"strconv"
	"sync"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/shynome/err0"
	"github.com/shynome/err0/try"
	"github.com/shynome/wgortc/bind"
	"github.com/shynome/wgortc/device/logger"
	"github.com/spf13/viper"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var devLocker sync.Mutex
var wgBind *bind.Bind
var wgConfig *Config
var wgHandler http.Handler

func GetRoutes() []string {
	return []string{
		viper.GetString("ip6_addr") + "/24",
		viper.GetString("ip4_route"),
	}
}

func InitIPC(app core.App) (err error) {
	defer err0.Then(&err, nil, nil)

	wgConfig = &Config{App: app}
	wgBind = bind.New(wgConfig)
	wgBind.SetLogger(app.Logger())
	wgHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wgBind.GetDevice() == nil {
			http.Error(w, "WireGuard 尚未启动", http.StatusServiceUnavailable)
			return
		}
		wgBind.ServeHTTP(w, r)
	})

	app.OnServe().BindFunc(func(se *core.ServeEvent) (err error) {
		defer err0.Then(&err, nil, nil)

		if keyStr := viper.GetString("wg_key"); keyStr == "" {
			key := try.To1(wgtypes.GeneratePrivateKey())
			keyStr = key.String()
			viper.Set("wg_key", keyStr)
			try.To(viper.SafeWriteConfig())
		}

		base := getBaseTry()
		wgConfig.base.Store(&base)

		se.Router.GET("/api/whip", func(e *core.RequestEvent) error {
			wgHandler.ServeHTTP(e.Response, e.Request)
			return nil
		})

		ipc := se.Router.Group("/api/ipc")
		ipc.BindFunc(func(e *core.RequestEvent) error {
			info, err := e.RequestInfo()
			if err != nil {
				return apis.NewBadRequestError("获取请求信息出错", err)
			}
			prod := !e.App.IsDev()
			if prod {
				if !info.HasSuperuserAuth() {
					return apis.NewUnauthorizedError("仅允许管理员请求该接口", nil)
				}
			}
			return e.Next()
		})

		ipc.GET("/device/android/start", func(e *core.RequestEvent) error {
			if mvpn == nil {
				return apis.NewApiError(http.StatusServiceUnavailable, "mvpn 尚未设置", nil)
			}
			mvpn.Start()
			return e.NoContent(http.StatusNoContent)
		})

		ipc.POST("/device", func(e *core.RequestEvent) error {
			if locked := devLocker.TryLock(); !locked {
				return apis.NewApiError(http.StatusLocked, "device 正在被操作中", nil)
			}
			defer devLocker.Unlock()

			var params DeviceParams
			if err := e.BindBody(&params); err != nil {
				return err
			}

			if err := startWireGuard(params); err != nil {
				return err
			}

			return e.JSON(http.StatusCreated, apis.NewApiError(http.StatusCreated, "启动成功", nil))
		})
		ipc.DELETE("/device", func(e *core.RequestEvent) error {
			if locked := devLocker.TryLock(); !locked {
				return apis.NewApiError(http.StatusLocked, "device 正在被操作中", nil)
			}
			defer devLocker.Unlock()

			dev := wgBind.GetDevice()
			if dev == nil {
				return apis.NewApiError(http.StatusServiceUnavailable, "device 尚未启动", nil)
			}
			dev.Close()
			wgBind.Device.Store(nil)
			return e.NoContent(http.StatusNoContent)
		})
		se.App.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
			if dev := wgBind.Device.Swap(nil); dev != nil {
				dev.Close()
			}
			return e.Next()
		})
		ipc.GET("/device", func(e *core.RequestEvent) error {
			base := wgConfig.base.Load()
			if base == nil {
				return apis.NewApiError(http.StatusServiceUnavailable, "服务尚未初始化成功", nil)
			}
			pubkey := wgtypes.Key(base.key).PublicKey()
			ds := DeviceStatus{
				Running: wgBind.GetDevice() != nil,
				Pubkey:  pubkey.String(),
				Routes:  GetRoutes(),
				Android: mvpn != nil,
			}
			return e.JSON(http.StatusOK, ds)
		})

		ipc.GET("/settings", func(e *core.RequestEvent) error {
			var s Settings
			if err := viper.Unmarshal(&s); err != nil {
				return err
			}
			s2 := SettingsWithRunning{
				Settings: s,
			}
			s2.Running = wgBind.GetDevice() != nil
			s2.MAC = getMAC()
			return e.JSON(http.StatusOK, s2)
		})
		ulaPrefix := netip.MustParsePrefix("fdd9:f8ff::/32")
		ipc.POST("/settings", func(e *core.RequestEvent) (err error) {
			defer err0.Then(&err, nil, nil)

			var s Settings
			try.To(e.BindBody(&s))
			oldListen := viper.GetString("listen")

			if s.Listen != oldListen {
				l, err := net.Listen("tcp", s.Listen)
				if err != nil {
					msg := fmt.Sprintf("%s 新的地址监听失败", s.Listen)
					return apis.NewApiError(http.StatusPreconditionFailed, msg, err)
				}
				l.Close()
			}

			if s.ULA != "fdd9:f800::1" {
				ip6, err := netip.ParseAddr(s.ULA)
				if err != nil {
					return apis.NewBadRequestError("解析ip6_addr失败", err)
				}
				if !ulaPrefix.Contains(ip6) {
					return apis.NewBadRequestError("IPv6唯一地址(ip6_addr)超出范围, 需要在 fdd9:f8ff::/32 范围内", nil)
				}
			}

			ms := map[string]any{}
			b := try.To1(json.Marshal(s))
			try.To(json.Unmarshal(b, &ms))
			for k, v := range ms {
				viper.Set(k, v)
			}
			try.To(viper.WriteConfig())

			if s.Listen != oldListen {
				event := &core.TerminateEvent{
					App:       e.App,
					IsRestart: true,
				}
				e.App.OnTerminate().Trigger(event, func(e *core.TerminateEvent) error {
					logger := e.App.Logger()
					RestartCh <- 1
					logger.Info("重启中")
					return e.Next()
				})
				return e.NoContent(http.StatusNoContent)
			}

			running := wgBind.GetDevice() != nil
			if running {
				StopWireGuard()
				if err := CommonStartWireGuard(); err != nil {
					return apis.NewInternalServerError(err.Error(), err)
				}
			} else {
				base := getBaseTry()
				wgConfig.base.Store(&base)
			}

			return e.NoContent(http.StatusNoContent)
		})

		return se.Next()
	})

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		if !viper.GetBool("auto_start") {
			return se.Next()
		}
		if err := CommonStartWireGuard(); err != nil {
			se.App.Logger().Error("auto_start failed", "error", err)
		}
		return se.Next()
	})

	return nil
}

var RestartCh = make(chan int)

// 支持 Android 端启动, 不用加锁
func CommonStartWireGuard() error {
	if mvpn != nil {
		mvpn.Start()
		return nil
	}
	return StartWireGuard(DeviceParams{})
}

type Settings struct {
	ULA       string `mapstructure:"ip6_addr" json:"ip6_addr" form:"ip6_addr"`
	Route     string `mapstructure:"ip4_route" json:"ip4_route" form:"ip4_route"`
	Listen    string `mapstructure:"listen" json:"listen" form:"listen"`
	Tun       string `mapstructure:"tun" json:"tun" form:"tun"`
	Key       string `mapstructure:"wg_key" json:"wg_key" form:"wg_key"`
	AutoStart bool   `mapstructure:"auto_start" json:"auto_start" form:"auto_start"`
}

type SettingsWithRunning struct {
	Settings
	Running bool   `json:"running"`
	MAC     string `json:"mac"`
}

func getMAC() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range ifaces {
		// 忽略回环接口和没有 MAC 地址的接口
		if iface.Flags&net.FlagLoopback == 0 && len(iface.HardwareAddr) != 0 {
			return iface.HardwareAddr.String()
		}
	}

	return ""
}

type DeviceStatus struct {
	Pubkey  string
	Routes  []string
	Running bool
	Android bool
}

func getBaseTry() configBase {
	viper.ReadInConfig() // 重新加载配置文件
	keyStr := viper.GetString("wg_key")
	key := device.NoisePrivateKey(decodeBase64(keyStr))
	routes := GetRoutes()
	_, portStr := try.To2(net.SplitHostPort(ListenAddr))
	port := try.To1(strconv.Atoi(portStr))

	base := configBase{
		key:  key,
		port: uint16(port),
	}
	for i, r := range routes {
		pf := try.To1(netip.ParsePrefix(r))
		base.dst[i] = pf.Addr()
	}
	return base
}

func startWireGuard(params DeviceParams) (err error) {
	defer err0.Then(&err, nil, nil)
	var app core.App = wgConfig.App
	dev := wgBind.GetDevice()
	if dev != nil {
		return apis.NewApiError(http.StatusOK, "device 已启动", nil)
	}
	defer err0.Then(&err, nil, func() {
		wgBind.Device.Store(nil) // 出错了的话将 device 重置为 nil
	})

	var cRouteUp = func(tun tun.Device, routes []string) (err error) {
		app.Logger().Error("此平台不支持RouteUp", "os", runtime.GOOS)
		return nil
	}
	if RouteUp != nil {
		cRouteUp = RouteUp
	}

	var tdev tun.Device
	switch {
	case params.FD != 0:
		tdev = try.To1(tunFromFD(params.FD))
	default:
		const MTU = 2400 // 2400 就是最适合 webrtc 的 mtu, webrtc 的 mtu 是 1200, 设置成 2400 刚好将一个包拆成两个
		tdev = try.To1(tun.CreateTUN(viper.GetString("tun"), MTU))
	}
	defer err0.Then(&err, nil, func() {
		tdev.Close() // 如果出错了, 释放资源
	})

	base := getBaseTry()
	wgConfig.base.Store(&base)
	logger := logger.New("net.remoon.well ")
	dev = device.NewDevice(tdev, wgBind, logger)
	ipcConf := try.To1(wgConfig.IpcConfig())
	try.To(dev.IpcSet(ipcConf))
	wgBind.Device.Store(dev)
	try.To(dev.Up())
	defer err0.Then(&err, nil, func() {
		dev.Close() // 如果出错了, 释放资源
	})

	if !params.Routed {
		routes := GetRoutes()
		try.To(cRouteUp(tdev, routes))
	}

	return nil
}

type DeviceParams struct {
	FD     int  `json:"fd"`     //
	Routed bool `json:"routed"` // 如果路由已经添加好了, 则不再次添加
}
