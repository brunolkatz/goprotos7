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
		migrateCreatePLCWatchLists_1(ctx),        // Create persistent PLC watch lists
		migrateCreateHeartbeatRegistrations_1(ctx),
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

func migrateCreateHeartbeatRegistrations_1(ctx context.Context) *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "migrateCreateHeartbeatRegistrations_1",
		Migrate: func(tx *gorm.DB) error {
			return tx.WithContext(ctx).Exec(`
				create table if not exists heartbeat_registrations
				(
					id               integer primary key autoincrement,
					db_number        integer not null,
					variable_name    text not null,
					address          text not null unique,
					start_on_connect integer not null default 1,
					enabled          integer not null default 1,
					heartbeat_type   text not null,
					alarm_seconds    integer not null default 5,
					last_value       integer,
					last_check_at    datetime,
					last_change_at   datetime,
					is_failing       integer not null default 0,
					last_error       text default '',
					created_at       datetime default current_timestamp,
					updated_at       datetime default current_timestamp
				);
			`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.WithContext(ctx).Exec(``).Error
		},
	}
}

func migrateCreatePLCWatchLists_1(ctx context.Context) *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "migrateCreatePLCWatchLists_1",
		Migrate: func(tx *gorm.DB) error {
			return tx.WithContext(ctx).Exec(`
			    -- Executing migrateCreatePLCWatchLists_1
				create table if not exists plc_watch_lists
				(
					source    text    not null,
					db_number integer not null default 0,
					addresses text    not null default '',
					primary key (source, db_number)
				);
			`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.WithContext(ctx).Exec(`
			    -- Executing Rollback for migrateCreatePLCWatchLists_1
			`).Error
		},
	}
}

func createDbVariablesTable_1(ctx context.Context) *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "createDbVariablesTable_1",
		Migrate: func(tx *gorm.DB) error {
			return tx.WithContext(ctx).Exec(`
			    -- Executing createDbVariablesTable_1
				create table db_variables
				(
					id          INTEGER
						primary key autoincrement,
					db_number   INTEGER               not null,
					name        TEXT                  not null,
					data_type   TEXT                  not null,
					byte_offset INTEGER               not null,
					bit_offset  INTEGER,
					length      INTEGER,
					description TEXT,
					real_val    real,
					int_val     integer,
					str_val     text,
					bool_val    integer,
					var_type    text default 'STATIC' not null
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
				create table static_var_definitions
				(
					id             integer            not null
						constraint static_var_definitions_pk
							primary key,
					db_variable_id integer            not null
						constraint static_var_definitions_db_variables_id_fk
							references db_variables
							on update cascade on delete cascade,
					description    text               not null,
					int_value      integer,
					float_value    integer,
					static_type    text default 'INT' not null,
					bit_offset     integer
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
