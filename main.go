package main

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/shynome/err0/try"
	"github.com/spf13/viper"
	_ "remoon.net/well/db/migrations"
	"remoon.net/well/wg"
)

var Version = "dev"

func main() {
	app := pocketbase.New()
	app.RootCmd.Version = Version

	ddir := app.DataDir()
	viper.SetConfigName("net.remoon.well")
	viper.SetConfigType("toml")
	viper.AddConfigPath(ddir)
	viper.SetDefault("ip4_route", "192.168.211.1/20")
	viper.SetDefault("listen", "127.0.0.1:7799")
	viper.SetDefault("tun", "well-net")
	viper.ReadInConfig()

	app.OnServe().BindFunc(func(e *core.ServeEvent) (err error) {
		addr := viper.GetString("listen")
		e.Server.Addr = addr
		return e.Next()
	})
	app.OnServe().BindFunc(wg.BindHook)
	app.OnServe().BindFunc(wg.BindIPC)
	try.To(app.Start())
}
