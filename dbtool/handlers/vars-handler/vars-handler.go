package vars_handler

import (
	"context"
	"errors"
	"fmt"
	"github.com/brunolkatz/goprotos7"
	"github.com/brunolkatz/goprotos7/dbtool"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	"github.com/brunolkatz/goprotos7/dbtool/db/sqlite_db"
	"github.com/charmbracelet/log"
	"gorm.io/gorm"
)

type dataBlocksHandler interface {
	WriteToDb(_ context.Context, dbVar *db_models.DbVariable) error
	CreateDatabaseBlocks() error
}

// VarsHandler - Implements all create/update/delete etc handlers for variables in the database.
type VarsHandler struct {
	db                *sql_lite_db.DB
	dataBlocksHandler dataBlocksHandler
	log               *log.Logger
}

func New(db *sql_lite_db.DB, dataBlocksHandler dataBlocksHandler, log *log.Logger) (*VarsHandler, error) {
	return &VarsHandler{
		db:                db,
		dataBlocksHandler: dataBlocksHandler,
		log:               log,
	}, nil
}

func (h *VarsHandler) GetDbVar(ctx context.Context, id int64) (*db_models.DbVariable, error) {
	return h.db.GetDBVarById(ctx, id)
}

func (h *VarsHandler) GetDbNumbers(ctx context.Context) ([]uint32, error) {
	return h.db.GetDBNumbers(ctx)
}

func (h *VarsHandler) GetVariables(dbNumber int32) ([]*db_models.DbVariable, error) {
	return h.db.GetVariables(context.Background(), dbNumber)
}

func (h *VarsHandler) CreateVariable(ctx context.Context, newVar *dbtool.CreateVarRequest) (*db_models.DbVariable, error) {

	lastVariable, err := h.db.GetLatestVariable(newVar.DBNumber)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("(CreateVariable) - Error getting max offset: %+v", err)
		return nil, err
	}
	offset := int64(0)
	if lastVariable != nil {
		offset = lastVariable.ByteOffset + int64(goprotos7.DataTypeSize[lastVariable.DataType])
		if lastVariable.DataType == goprotos7.STRING { // Add the string length to the offset
			offset += int64(*lastVariable.Length)
		}
	}

	var ret *db_models.DbVariable
	err = h.db.DbConn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		var l *uint8
		if newVar.DataType == goprotos7.STRING {
			if newVar.StrLength == nil {
				return fmt.Errorf("string length is required for STRING data type")
			}
			l = newVar.StrLength
		} else {
			l = nil
		}

		tVar := &db_models.DbVariable{
			DbNumber:    newVar.DBNumber,
			Name:        newVar.Name,
			DataType:    newVar.DataType,
			ByteOffset:  offset,
			BitOffset:   nil,
			Length:      l,
			Description: newVar.Description,
			VarType:     dbtool.VarTypeList,
		}

		var intVal *int64
		var strVal *string
		var floatVal *float64
		var boolVal *bool
		switch newVar.DataType {
		case goprotos7.STRING, goprotos7.CHAR:
			if newVar.StringVal == nil {
				return fmt.Errorf("string value is required for STRING data type")
			}
			if newVar.DataType == goprotos7.CHAR {
				if len(*newVar.StringVal) > 1 {
					return fmt.Errorf("CHAR data type can only hold a single character, got: %s", *newVar.StringVal)
				}
			}
			strVal = newVar.StringVal
			tVar.VarType = dbtool.VarTypeStatic
		case goprotos7.BOOL: // For boolean value we create a list of db_variables with a static TRUE and FALSE values at static_var_definitions table
			if newVar.ListFields == nil || len(newVar.ListFields) == 0 {
				return fmt.Errorf("list fields are required for BOOL data type")
			}
			if len(newVar.ListFields) > 8 {
				return fmt.Errorf("too many BOOL fields, maximum is 8 (one for each bit in a byte)")
			}

			for _, boolVar := range newVar.ListFields {
				tBoolVar := &db_models.DbVariable{
					DbNumber:    newVar.DBNumber,
					Name:        fmt.Sprintf("%s_%s", newVar.Name, boolVar.Description),
					DataType:    goprotos7.BOOL,
					ByteOffset:  offset, // Offset is the same for all BOOL variables in the list since they are part of the same byte value (8 bits flags)
					BitOffset:   boolVar.BitOffset,
					Length:      nil,
					Description: boolVar.Description,
					VarType:     dbtool.VarTypeList, // Boolean variables are always of type LIST of TRUE and FALSE at static_var_definitions table
				}

				tErr := tx.WithContext(ctx).Create(tBoolVar).Error
				if tErr != nil {
					return tErr
				}

				for _, i := range []bool{true, false} {
					tStaticVarDef := db_models.StaticVarDefinition{
						DbVariableId: tBoolVar.Id,
						StaticType:   dbtool.StaticTypeBool, // TODO: add more static types
						IntValue:     nil,
						FloatValue:   nil,
					}
					_i := int64(0)
					desc := ""
					if i {
						_i = 1
						desc = "TRUE"
					} else {
						_i = 0
						desc = "FALSE"
					}
					tStaticVarDef.IntValue = &_i
					tStaticVarDef.Description = desc
					err := tx.WithContext(ctx).Create(&tStaticVarDef).Error
					if err != nil {
						return fmt.Errorf("error creating static variable definition: %w", err)
					}
				}
			}
			return nil // Exit early after creating the BOOL variable
		case goprotos7.BYTE, goprotos7.WORD, goprotos7.DWORD, goprotos7.LWORD, goprotos7.SINT, goprotos7.USINT, goprotos7.INT, goprotos7.UINT, goprotos7.DINT, goprotos7.UDINT, goprotos7.LINT:
			if newVar.IntVal == nil {
				return fmt.Errorf("integer value is required for %s data type", newVar.DataType)
			}
			intVal = newVar.IntVal
			tVar.VarType = dbtool.VarTypeList
		case goprotos7.REAL, goprotos7.LREAL:
			if newVar.FloatVal == nil {
				return fmt.Errorf("float value is required for %s data type", newVar.DataType)
			}
			floatVal = newVar.FloatVal
			tVar.VarType = dbtool.VarTypeList
		default:
			return fmt.Errorf("unsupported data type: %s", newVar.DataType)
		}

		tVar.IntVal = intVal
		tVar.FloatVal = floatVal
		tVar.StringVal = strVal
		tVar.BoolVal = boolVal

		tErr := tx.WithContext(ctx).Create(tVar).Error
		if tErr != nil {
			return tErr
		}

		switch tVar.VarType {
		case dbtool.VarTypeStatic:
		case dbtool.VarTypeList:
			if newVar.ListFields == nil || len(newVar.ListFields) == 0 {
				return fmt.Errorf("list values are required for LIST data type")
			}
			for _, field := range newVar.ListFields {
				tStaticVarDef := db_models.StaticVarDefinition{
					DbVariableId: tVar.Id,
					Description:  field.Description,
					IntValue:     field.IntValue,
					FloatValue:   nil,
					StaticType:   dbtool.StaticTypeInt,
				}
				err := tx.WithContext(ctx).Create(&tStaticVarDef).Error
				if err != nil {
					return fmt.Errorf("error creating static variable definition: %w", err)
				}
			}
		default:
			return fmt.Errorf("unsupported data type: %s", newVar.DataType)
		}

		ret = tVar // assign the created variable to ret
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Recreate the database blocks after creating the variable
	err = h.dataBlocksHandler.CreateDatabaseBlocks()
	if err != nil {
		log.Errorf("(CreateVariable) - Error creating database blocks: %+v", err)
		return nil, fmt.Errorf("error creating database blocks: %w", err)
	}

	return ret, nil
}

// SetListVar - Change the activated value to the given value for a variable of type LIST.
func (h *VarsHandler) SetListVar(ctx context.Context, dbNumber, varId, stsId int64) (*db_models.DbVariable, error) {

	dbVar, err := h.db.GetDBVarById(ctx, varId)
	if err != nil {
		return nil, err
	}
	if dbVar == nil {
		return nil, fmt.Errorf("variable with id %d not found", varId)
	}
	if dbVar.DbNumber != dbNumber {
		return nil, fmt.Errorf("variable with id %d does not belong to db number %d", varId, dbNumber)
	}
	stsDef, err := h.db.GetStaticVarDefinitionById(ctx, varId, stsId)
	if err != nil {
		return nil, fmt.Errorf("error getting static variable definition by id %d: %w", stsId, err)
	}

	switch stsDef.StaticType {
	case dbtool.StaticTypeInt:
		dbVar.IntVal = stsDef.IntValue
	case dbtool.StaticTypeFloat:
		dbVar.FloatVal = stsDef.FloatValue
	case dbtool.StaticTypeBool:
		b := false
		if stsDef.IntValue != nil {
			if *stsDef.IntValue == 1 {
				b = true
			}
		}
		dbVar.BoolVal = &b
	default:
		return nil, fmt.Errorf("unsupported LIST type: %s", stsDef.StaticType)
	}

	err = h.db.DbConn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tErr := tx.
			Model(db_models.DbVariable{}).
			Where("id = ?", dbVar.Id).
			Save(dbVar).Error
		if tErr != nil {
			return fmt.Errorf("error updating variable: %w", tErr)
		}
		dbVar.UpdateIsSelected() // Reset the selection state of static variable definitions and set again
		return nil
	})
	if err != nil {
		log.Errorf("(SetListVar) - Error updating variable: %+v", err)
		return nil, fmt.Errorf("error updating variable: %w", err)
	}

	// Write to the file
	err = h.dataBlocksHandler.WriteToDb(ctx, dbVar)
	if err != nil {
		log.Errorf("(SetListVar) - Error writing variable to file: %+v", err)
		return nil, fmt.Errorf("error writing variable to file: %w", err)
	}
	return dbVar, nil
}
