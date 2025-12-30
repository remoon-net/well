package wg

import (
	"net/http"
	"net/netip"
	"runtime"
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

var dev *device.Device
var devLocker sync.Mutex

func BindIPC(se *core.ServeEvent) (err error) {
	defer err0.Then(&err, nil, nil)

	if keyStr := viper.GetString("wg_key"); keyStr == "" {
		key := try.To1(wgtypes.GeneratePrivateKey())
		keyStr = key.String()
		viper.Set("wg_key", keyStr)
		try.To(viper.SafeWriteConfig())
	}

	ipc := se.Router.Group("/api/ipc")
	ipc.BindFunc(func(e *core.RequestEvent) error {
		info, err := e.RequestInfo()
		if err != nil {
			return apis.NewBadRequestError("获取请求信息出错", err)
		}
		if false && !info.HasSuperuserAuth() {
			return apis.NewUnauthorizedError("仅允许管理员请求该接口", nil)
		}
		if locked := devLocker.TryLock(); !locked {
			return apis.NewApiError(http.StatusLocked, "device 正在被操作中", nil)
		}
		defer devLocker.Unlock()
		return e.Next()
	})

	getRoutes := func() []string {
		return []string{
			"fdd9:f800::1/24",
			viper.GetString("ip4_route"),
		}
	}

	ipc.GET("/device/routes", func(e *core.RequestEvent) error {
		// 给安卓端用的, 安卓端路由必须在 builder.establish() 之前设定好之后才有 tun, 需要分两步
		return e.JSON(http.StatusOK, getRoutes())
	})

	ipc.POST("/device", func(e *core.RequestEvent) (err error) {
		defer err0.Then(&err, nil, nil)

		if dev != nil {
			return apis.NewApiError(http.StatusOK, "device 已启动", nil)
		}

		var params DeviceParams
		try.To(e.BindBody(&params))

		viper.ReadInConfig() // 重新加载配置文件
		keyStr := viper.GetString("wg_key")
		key := device.NoisePrivateKey(decodeBase64(keyStr))
		routes := getRoutes()

		c := &Config{
			App: e.App,
			key: key,
		}
		for i, r := range routes {
			pf := try.To1(netip.ParsePrefix(r))
			c.dst[i] = pf.Addr()
		}

		var cRouteUp = func(tun tun.Device, routes []string) (err error) {
			e.App.Logger().Error("此平台不支持RouteUp", "os", runtime.GOOS)
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

		b := bind.New(c)
		logger := logger.New("net.remoon.well ")
		dev = device.NewDevice(tdev, b, logger)
		b.Device.Store(dev)
		try.To(dev.Up())

		try.To(cRouteUp(tdev, routes))

		return e.String(http.StatusCreated, "启动成功")
	})
	ipc.DELETE("/device", func(e *core.RequestEvent) error {
		if dev == nil {
			return apis.NewApiError(http.StatusServiceUnavailable, "device 尚未启动", nil)
		}
		dev.Close()
		dev = nil
		return e.NoContent(http.StatusNoContent)
	})
	ipc.GET("/device", func(e *core.RequestEvent) error {
		if dev == nil {
			return apis.NewApiError(http.StatusServiceUnavailable, "device 尚未启动", nil)
		}
		s, err := dev.IpcGet()
		if err != nil {
			return apis.NewApiError(http.StatusInternalServerError, err.Error(), err)
		}
		return e.String(http.StatusOK, s)
	})
	return se.Next()
}

type DeviceParams struct {
	FD int `json:"fd"`
}
