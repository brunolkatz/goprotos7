package sql_lite_db

import (
	"context"
	"github.com/brunolkatz/goprotos7/webadmin/db/db_models"
)

func (d *DB) GetDBNumbers(ctx context.Context) ([]uint32, error) {
	type Ret struct {
		DbNumber uint32 `gorm:"column:db_number" db:"db_number" json:"db_number"`
	}
	var ret []*Ret
	err := d.DbConn.
		WithContext(ctx).
		Model(db_models.DbVariables{}).
		Select("db_number").
		Group("db_number").
		Order("db_number ASC").
		Find(&ret).Error
	if err != nil {
		return nil, err
	}
	if len(ret) == 0 {
		return make([]uint32, 0), nil
	}
	var dbNumbers []uint32
	for _, r := range ret {
		dbNumbers = append(dbNumbers, r.DbNumber)
	}
	return dbNumbers, nil
}

func (d *DB) GetDbVariables(ctx context.Context, dbNumber uint32) ([]*db_models.DbVariables, error) {
	var ret []*db_models.DbVariables
	err := d.DbConn.
		WithContext(ctx).
		Model(db_models.DbVariables{}).
		Where("db_number = ?", dbNumber).
		Order("byte_offset ASC").
		Find(&ret).Error
	if err != nil {
		return nil, err
	}
	return ret, nil
}
