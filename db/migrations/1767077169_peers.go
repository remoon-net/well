package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
	"github.com/shynome/err0"
	"github.com/shynome/err0/try"
	"remoon.net/well/db"
)

func init() {
	migrations.Register(func(app core.App) (err error) {
		defer err0.Then(&err, nil, nil)

		ices := core.NewBaseCollection(db.TableICEs, ID(db.TableICEs))
		ices.Fields.Add(
			&core.TextField{
				Name: "name", Id: ID("name"), System: true,
				Required: false, Presentable: true,
			},
			&core.TextField{
				Name: "urls", Id: ID("urls"), System: true,
				Required: true,
			},
			&core.TextField{
				Name: "username", Id: ID("username"), System: true,
				Required: false,
			},
			&core.TextField{
				Name: "credential", Id: ID("credential"), System: true,
				Required: false,
			},
			&core.BoolField{
				Name: "default", Id: ID("default"), System: true,
				Required: false,
			},
		)
		addUpdatedFields(ices)
		try.To(app.Save(ices))

		modes := []string{
			db.TransportModeNoWS,
			db.TransportModeNoWebRTC,
			db.TransportModeWSRedirect,
		}
		peers := core.NewBaseCollection(db.TablePeers, ID(db.TablePeers))
		peers.Fields.Add(
			&core.TextField{
				Name: "name", Id: ID("name"), System: true,
				Required: false, Presentable: true,
			},
			&core.BoolField{
				Name: "disabled", Id: ID("disabled"), System: true,
				Required: false,
			},
			&core.DateField{
				Name: "handshaked", Id: ID("handshaked"), System: true,
				Required: false,
			},
			&core.TextField{
				Name: "pubkey", Id: ID("pubkey"), System: true,
				Required: true,
			},
			&core.SelectField{
				Name: "transport_mode", Id: ID("transport_mode"), System: true,
				Required: false,
				Values:   modes, MaxSelect: len(modes),
			},
			&core.TextField{
				Name: "psk", Id: ID("psk"), System: true,
				Required: false,
			},
			&core.RelationField{
				Name: "ices", Id: ID("ices"), System: true,
				Required:     false,
				CollectionId: ices.Id, MaxSelect: 999,
			},
			&core.TextField{
				Name: "ipv4", Id: ID("ipv4"), System: true,
				Required: false,
			},
			&core.NumberField{
				Name: "ip_num", Id: ID("ip_num"), System: true,
				Required: false,
				OnlyInt:  true,
			},
			&core.TextField{
				Name: "ipv6", Id: ID("ipv6"), System: true,
				Required: false,
			},
			&core.URLField{
				Name: "endpoint", Id: ID("endpoint"), System: true,
				Required: false,
			},
			&core.URLField{
				Name: "endpoint2", Id: ID("endpoint2"), System: true,
				Required: false,
			},
			&core.NumberField{
				Name: "auto", Id: ID("auto"), System: true,
				Required: false,
				OnlyInt:  true,
			},
		)
		addUpdatedFields(peers)
		try.To(app.Save(peers))

		return nil
	}, func(app core.App) error {
		return fmt.Errorf("no rollback")
	})
}
