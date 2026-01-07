package cmd

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	webui "remoon.net/well-webui"
)

var uiHandler = &hook.Handler[*core.ServeEvent]{
	Func: func(se *core.ServeEvent) error {
		se.Router.GET("/{path...}", apis.Static(webui.FS, true))
		return se.Next()
	},
	Priority: 999, // execute as latest as possible to allow users to provide their own route
}
