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

		linkers := core.NewBaseCollection(db.TableLinkers, ID(db.TableLinkers))
		linkers.Fields.Add(
			&core.TextField{
				Name: "name", Id: ID("name"), System: true,
				Required: false, Presentable: true,
			},
			&core.BoolField{
				Name: "disabled", Id: ID("disabled"), System: true,
				Required: false,
			},
			&core.TextField{
				Name: "status", Id: ID("status"), System: true,
				Required: false,
			},
			&core.URLField{
				Name: "linker", Id: ID("linker"), System: true,
				Required: true,
			},
			&core.URLField{
				Name: "whip", Id: ID("whip"), System: true,
				Required: false,
			},
		)
		addUpdatedFields(linkers)
		try.To(app.Save(linkers))

		return nil
	}, func(app core.App) error {
		return fmt.Errorf("no rollback")
	})
}
