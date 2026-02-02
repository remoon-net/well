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

		peers := try.To1(app.FindCollectionByNameOrId(db.TablePeers))
		peers.Fields.AddAt(getFieldNext(peers, "ipv6"),
			&core.NumberField{
				Name: "ip6_num", Id: ID("ip6_num"), System: true,
				Required: false,
				OnlyInt:  true,
			},
		)
		try.To(app.Save(peers))

		return nil
	}, func(app core.App) error {
		return fmt.Errorf("no rollback")
	})
}
