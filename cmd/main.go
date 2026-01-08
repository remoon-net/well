package cmd

import (
	"encoding/json"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/shynome/err0"
	"github.com/shynome/err0/try"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"remoon.net/well/db"
	_ "remoon.net/well/db/migrations"
	"remoon.net/well/wg"
)

var Version = "dev"

var ExitCh = make(chan int)

func Main(argsStr string) string {
	var args []string
	if argsStr != "" {
		try.To(json.Unmarshal([]byte(argsStr), &args))
		os.Args = append(os.Args, args...)
	}
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
		e.InstallerFunc = func(app core.App, systemSuperuser *core.Record, baseURL string) (err error) {
			defer err0.Then(&err, nil, nil)

			superusers := try.To1(app.FindCachedCollectionByNameOrId("_superusers"))
			su := core.NewRecord(superusers)
			su.SetEmail("well@remoon.net")
			su.SetPassword("well@remoon.net")
			try.To(app.Save(su))

			ices := try.To1(app.FindCachedCollectionByNameOrId(db.TableICEs))
			ice := core.NewRecord(ices)
			ice.Load(map[string]any{
				"name":    "remoon",
				"urls":    "stun:stun.remoon.net:80",
				"default": true,
			})
			try.To(app.Save(ice))

			return nil
		}
		serveCmd := getServeCmd(app)
		listenFlag := serveCmd.Flag("http")
		if listenFlag.Changed {
			wg.ListenAddr = listenFlag.Value.String()
			return e.Next()
		}
		addr := viper.GetString("listen")
		e.Server.Addr = addr
		wg.ListenAddr = addr
		return e.Next()
	})
	try.To(wg.InitHook(app))
	try.To(wg.InitIPC(app))
	try.To(wg.InitLinkers(app))
	app.OnServe().Bind(uiHandler)

	firstbootCh := make(chan error)
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Id: "firstboot",
		Func: func(e *core.ServeEvent) error {
			if err := e.Next(); err != nil {
				firstbootCh <- err
				return err
			}
			firstbootCh <- nil
			return nil
		},
	})
	defer app.OnServe().Unbind("firstboot")

	finished := make(chan int, 2)
	finished <- 1
	go func() {
		logger := app.Logger()
		for range wg.RestartCh {
			<-finished
			err := app.Start()
			if err != nil {
				logger.Error("程序退出", "error", err)
			}
			if wg.ListenAddr == "" {
				firstbootCh <- nil
				ExitCh <- 1
			}
			finished <- 1
		}
	}()
	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)
		<-sigch
		<-finished // 等待 app exit
		ExitCh <- 0
	}()

	wg.RestartCh <- 1
	err := <-firstbootCh
	try.To(err)
	return wg.ListenAddr
}

func getServeCmd(app *pocketbase.PocketBase) *cobra.Command {
	cmds := app.RootCmd.Commands()
	for _, cmd := range cmds {
		if strings.HasPrefix(cmd.Use, "serve ") {
			return cmd
		}
	}
	return nil
}
