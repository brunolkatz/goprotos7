package webadmin

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrSQLiteMigrationFailed = errors.New("SQLite migration failed")
)

const (
	DefaultWebAdminHTMLTitle = "GoSimpleHTMX"
)

func GetSubPageWebAdminTitle(title string) string {
	return fmt.Sprintf("%s - %s", title, DefaultWebAdminHTMLTitle)
}

func IsHTMXReq(r *http.Request) bool {
	return r.Header.Get("HX-Request") != ""
}

type Config struct {
	SQLiteFilePath string `long:"sqlite-path" env:"SQLITE_PATH" default:"data.db" short:"s" description:"SQLite location path string, including the name of the database. Default database name will be 'data.db' and will be created at the executable path if this argument is not provided"`
	DBBinPaths     string `long:"db-bin-path" env:"DB_BIN_PATH" default:"" short:"b" description:"Path where the bin files will be created. If empty, the \"pwd\" path will be applied."`

	Flags struct {
		DisableWebAdmin bool `long:"disable-webadmin" env:"DISABLE_WEBADMIN" description:"Disable the web admin interface, only will recreate the data bin blocks"`
	} `group:"flags" namespace:"flags" env-namespace:"FLAGS" description:""`

	LogLevel struct {
		SQLite string `default:"SILENCE" env:"SQLITE" long:"sqlite" short:"i" description:"The log level for the sqlite database. Default values will be applied if no value is given." choise:"ERROR" choise:"INFO" choise:"SILENCE"`
	} `group:"log-level" namespace:"log-level" env-namespace:"LOG_LEVEL" description:"Log level for the web admin interface"`
}
