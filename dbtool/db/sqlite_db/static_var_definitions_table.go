package sql_lite_db

import (
	"context"
	"errors"
	"fmt"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	"gorm.io/gorm"
)

func (d *DB) GetStaticVarDefinitionById(ctx context.Context, varId, stsId int64) (*db_models.StaticVarDefinition, error) {
	var ret *db_models.StaticVarDefinition
	err := d.DbConn.
		WithContext(ctx).
		Model(db_models.StaticVarDefinition{}).
		Where("db_variable_id = ? AND id = ?", varId, stsId).
		First(&ret).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("static variable definition with id %d not found for variable %d", stsId, varId)
		}
		return nil, fmt.Errorf("error fetching static variable definition: %w", err)
	}
	return ret, nil
}
