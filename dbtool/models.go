package dbtool

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrSQLiteMigrationFailed = errors.New("SQLite migration failed")
)

type VarType uint32

func (v VarType) String() string {
	if str, ok := VarTypeDepara[v]; ok {
		return str
	}
	return "UNKNOWN"
}

const (
	VarTypeStatic VarType = iota // VarTypeStatic is a static variable, it can be a string, int, float, etc. the value can't be changed after creation
	VarTypeList                  // VarTypeList variable has a of list values, it can be true or false or a list of values to be "toggled", used in BOOL or INT's DataType variables
)

var (
	VarTypeDepara = map[VarType]string{
		VarTypeStatic: "STATIC",
		VarTypeList:   "LIST",
	}
	VarTypePara = map[string]VarType{
		"STATIC": VarTypeStatic,
		"LIST":   VarTypeList,
	}
)

type StaticType string

const (
	StaticTypeInt    StaticType = "INT"    // StaticTypeInt is a static type for int values
	StaticTypeFloat  StaticType = "FLOAT"  // StaticTypeFloat is a static type for float values
	StaticTypeString StaticType = "STRING" // StaticTypeString is a static type for string values
	StaticTypeBool   StaticType = "BOOL"   // StaticTypeBool is a static type for boolean values
)

const (
	DefaultWebAdminHTMLTitle = "DBToolsWebAdmin"
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
		EnableWebAdmin bool `long:"enable-web-admin" env:"ENABLE_WEB_ADMIN" description:"Disable the web admin interface, only will recreate the data bin blocks"`
	} `group:"flags" namespace:"flags" env-namespace:"FLAGS" description:""`

	LogLevel struct {
		SQLite string `default:"SILENCE" env:"SQLITE" long:"sqlite" short:"i" description:"The log level for the sqlite database. Default values will be applied if no value is given." choise:"ERROR" choise:"INFO" choise:"SILENCE"`
	} `group:"log-level" namespace:"log-level" env-namespace:"LOG_LEVEL" description:"Log level for the web admin interface"`
}
