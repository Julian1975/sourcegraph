package migrations

import (
	"context"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const changesetSpecMigrationCountPerRun = 100

type changesetSpecMigrator struct {
	store *store.Store
}

var _ oobmigration.Migrator = &changesetSpecMigrator{}

func (m *changesetSpecMigrator) Progress(ctx context.Context) (float64, error) {
	progress, _, err := basestore.ScanFirstFloat(
		m.store.Query(ctx, sqlf.Sprintf(changesetSpecMigratorProgressQuery)),
	)
	if err != nil {
		return 0, err
	}

	return progress, nil
}

const changesetSpecMigratorProgressQuery = `
-- source: enterprise/cmd/worker/internal/batches/migrations/changeset_spec_migrator.go:Progress
SELECT CASE c2.count WHEN 0 THEN 1 ELSE CAST((c2.count - c1.count) AS float) / CAST(c2.count AS float) END FROM
	(SELECT COUNT(*) as count FROM changeset_specs WHERE NOT migrated) c1,
	(SELECT COUNT(*) as count FROM changeset_specs) c2
`

func (m *changesetSpecMigrator) Up(ctx context.Context) error {
	tx, err := m.store.Transact(ctx)
	if err != nil {
		return errors.Wrap(err, "starting transaction")
	}

	f := func() error {
		specs, _, err := tx.ListChangesetSpecs(ctx, store.ListChangesetSpecsOpts{
			LimitOpts:         store.LimitOpts{Limit: changesetSpecMigrationCountPerRun},
			RequiresMigration: true,
		})
		if err != nil {
			return errors.Wrap(err, "listing changeset specs")
		}
		for _, cs := range specs {
			if err := tx.MigrateChangesetSpec(ctx, cs); err != nil {
				return errors.Wrap(err, "migrating changeset spec")
			}
		}

		return nil
	}
	return tx.Done(f())
}

func (m *changesetSpecMigrator) Down(ctx context.Context) error {
	return m.store.Exec(ctx, sqlf.Sprintf(changesetSpecMigratorDownQuery))
}

const changesetSpecMigratorDownQuery = `
-- source: enterprise/cmd/worker/internal/batches/migrations/changeset_spec_migrator.go:Down
UPDATE
	changeset_specs
SET
	migrated = FALSE
WHERE
	1 = 1
`
