package sql_lite_db

import (
	"context"
	"strings"

	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
)

func (d *DB) SavePLCWatchList(ctx context.Context, source string, dbNumber int32, addresses []string) error {
	row := db_models.PLCWatchList{
		Source:    strings.TrimSpace(source),
		DBNumber:  dbNumber,
		Addresses: strings.Join(addresses, "\n"),
	}
	return d.DbConn.WithContext(ctx).Model(db_models.PLCWatchList{}).Save(&row).Error
}

func (d *DB) GetPLCWatchList(ctx context.Context, source string, dbNumber int32) ([]string, error) {
	var row db_models.PLCWatchList
	err := d.DbConn.WithContext(ctx).
		Model(db_models.PLCWatchList{}).
		Where("source = ? AND db_number = ?", strings.TrimSpace(source), dbNumber).
		First(&row).Error
	if err != nil {
		return nil, err
	}
	parts := strings.Split(row.Addresses, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out, nil
}
