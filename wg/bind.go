package wg

import (
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
		"fdd9:f800::1/24",
		viper.GetString("ip4_route"),
	}
}

func BindIPC(se *core.ServeEvent) (err error) {
	defer err0.Then(&err, nil, nil)

	if keyStr := viper.GetString("wg_key"); keyStr == "" {
		key := try.To1(wgtypes.GeneratePrivateKey())
		keyStr = key.String()
		viper.Set("wg_key", keyStr)
		try.To(viper.SafeWriteConfig())
	}

	wgConfig = &Config{App: se.App}
	base := getBaseTry()
	wgConfig.base.Store(&base)

	wgBind = bind.New(wgConfig)
	wgBind.SetLogger(se.App.Logger())
	wgHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wgBind.GetDevice() == nil {
			http.Error(w, "WireGuard 尚未启动", http.StatusServiceUnavailable)
			return
		}
		wgBind.ServeHTTP(w, r)
	})

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
		if locked := devLocker.TryLock(); !locked {
			return apis.NewApiError(http.StatusLocked, "device 正在被操作中", nil)
		}
		defer devLocker.Unlock()
		return e.Next()
	})

	ipc.GET("/device/android/start", func(e *core.RequestEvent) error {
		if mvpn == nil {
			return apis.NewApiError(http.StatusServiceUnavailable, "mvpn 尚未设置", nil)
		}
		mvpn.Start()
		return e.NoContent(http.StatusNoContent)
	})

	ipc.POST("/device", func(e *core.RequestEvent) (err error) {
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
		}
		return e.JSON(http.StatusOK, ds)
	})

	func() {
		if !viper.GetBool("auto_start") {
			return
		}
		devLocker.Lock()
		defer devLocker.Unlock()
		err := startWireGuard(DeviceParams{})
		try.To(err)
	}()

	return se.Next()
}

type DeviceStatus struct {
	Pubkey  string
	Routes  []string
	Running bool
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
