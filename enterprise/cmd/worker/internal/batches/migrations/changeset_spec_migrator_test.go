package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	bt "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	et "github.com/sourcegraph/sourcegraph/internal/encryption/testing"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/batches"
)

func TestChangesetSpecMigrator(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(logger, t))

	repo, _ := bt.CreateTestRepo(t, ctx, db)

	bstore := store.New(db, &observation.TestContext, et.TestKey{})

	migrator := &changesetSpecMigrator{bstore}

	t.Run("no records", func(t *testing.T) {
		assertProgress(t, ctx, 1.0, migrator)
	})

	// Create changeset specs to migrate.
	for i := 0; i < 2*changesetSpecMigrationCountPerRun; i++ {
		spec, err := json.Marshal(&batches.ChangesetSpec{ExternalID: fmt.Sprintf("id-%d", i+1)})
		if err != nil {
			t.Fatal(err)
		}
		if err := bstore.Exec(ctx, sqlf.Sprintf(`INSERT INTO changeset_specs (rand_id, spec, repo_id, migrated, diff_stat_added, diff_stat_changed, diff_stat_deleted) VALUES (%s, %s, %s, FALSE, 0, 0, 0)`, fmt.Sprintf("ID%d", i), spec, repo.ID)); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("completely unmigrated", func(t *testing.T) {
		assertProgress(t, ctx, 0.0, migrator)
	})

	t.Run("first migrate up", func(t *testing.T) {
		if err := migrator.Up(ctx); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		assertProgress(t, ctx, 0.5, migrator)
	})

	t.Run("second migrate up", func(t *testing.T) {
		if err := migrator.Up(ctx); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		assertProgress(t, ctx, 1.0, migrator)
	})

	t.Run("migrate down", func(t *testing.T) {
		if err := migrator.Down(ctx); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		assertProgress(t, ctx, 0.0, migrator)
	})
}
