package sql_lite_db

import (
	"context"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// runMigrations runs gorm Migrations
func (d *DB) runMigrations(ctx context.Context) error {
	if d.DbConn == nil {
		d.log.Errorf("[DB_SQLITE] Could not run migrations: DB connection is nil")
		return nil
	}

	migrations := []*gormigrate.Migration{
		createDbVariablesTable_1(ctx),            // Create the db_variables table
		migrateCreateStaticVarDefinitions_1(ctx), // Create the int_var_definitions table
	}
	if len(migrations) == 0 {
		d.log.Infof("[DB_SQLITE] No migrations found")
		return nil
	}
	migrator := gormigrate.New(d.DbConn, gormigrate.DefaultOptions, migrations)
	if err := migrator.Migrate(); err != nil {
		d.log.Errorf("[DB_SQLITE] Could not migrate: %v", err)
		return err
	}

	d.log.Infof("[DB_SQLITE] Migration did run successfully")
	return nil
}

func createDbVariablesTable_1(ctx context.Context) *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "createDbVariablesTable_1",
		Migrate: func(tx *gorm.DB) error {
			return tx.WithContext(ctx).Exec(`
			    -- Executing createDbVariablesTable_1
			    create table if not exists db_variables (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					db_number INTEGER NOT NULL,         -- Which data block it belongs to
					name TEXT NOT NULL,                 -- Variable name
					data_type TEXT NOT NULL,            -- S7 data type (e.g., BOOL, INT, REAL, STRING, ARRAY[0..9] OF INT)
					byte_offset INTEGER NOT NULL,       -- Byte offset in the DB
					bit_offset INTEGER,                 -- Only used for BOOL, NULL otherwise
					length INTEGER,                     -- Optional, used for STRING or ARRAY
					description TEXT                    -- Optional comment
				);
			`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.WithContext(ctx).Exec(`
			    -- Executing Rollback for createDbVariablesTable_1
			`).Error
		},
	}
}

func migrateCreateStaticVarDefinitions_1(ctx context.Context) *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "migrateCreateStaticVarDefinitions_1",
		Migrate: func(tx *gorm.DB) error {
			return tx.WithContext(ctx).Exec(`
			    -- Executing migrateCreateStaticVarDefinitions_1
				create table if not exists static_var_definitions
				(
					id             integer not null
						constraint static_var_definitions_pk
							primary key,
					db_variable_id integer not null
						constraint static_var_definitions_db_variables_id_fk
							references db_variables
							on update cascade on delete cascade,
					description    text    not null,
					int_value      integer,
					float_value    integer
				);

			`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.WithContext(ctx).Exec(`
			    -- Executing Rollback for migrateCreateStaticVarDefinitions_1
			`).Error
		},
	}
}
