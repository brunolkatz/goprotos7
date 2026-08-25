package create_var_api

import (
	"fmt"
	"github.com/brunolkatz/goprotos7"
	"github.com/brunolkatz/goprotos7/dbtool"
	"math"
	"net/http"
	"strconv"
	"strings"
)

func bindCreateVarRequest(r *http.Request) (*dbtool.CreateVarRequest, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("error parsing form: %w", err)
	}

	dbNumber, err := parseDBNumber(r)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(r.Form.Get("name"))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	description := strings.TrimSpace(r.Form.Get("description"))
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	dataTypeRaw := r.Form.Get("data_type")
	if dataTypeRaw == "" {
		return nil, fmt.Errorf("data_type is required")
	}
	dataTypeVal, err := strconv.ParseUint(dataTypeRaw, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid data type provided: %w", err)
	}

	req := &dbtool.CreateVarRequest{
		DBNumber:    dbNumber,
		Name:        name,
		Description: description,
		DataType:    goprotos7.DataType(dataTypeVal),
	}

	switch req.DataType {
	case goprotos7.BOOL:
		if err := fillBoolForm(req, r); err != nil {
			return nil, err
		}
	case goprotos7.STRING:
		if err := fillStringForm(req, r); err != nil {
			return nil, err
		}
	case goprotos7.CHAR:
		if err := fillCharForm(req, r); err != nil {
			return nil, err
		}
	case goprotos7.BYTE, goprotos7.WORD, goprotos7.DWORD, goprotos7.LWORD, goprotos7.SINT, goprotos7.USINT, goprotos7.INT, goprotos7.UINT, goprotos7.DINT, goprotos7.UDINT, goprotos7.LINT, goprotos7.ULINT:
		if err := fillIntegerListForm(req, r); err != nil {
			return nil, err
		}
	case goprotos7.REAL, goprotos7.LREAL:
		if err := fillFloatListForm(req, r); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported data type")
	}

	return req, nil
}

func parseDBNumber(r *http.Request) (int64, error) {
	mode := strings.TrimSpace(r.Form.Get("db-number-mode"))
	switch mode {
	case "new":
		return parseRequiredInt64(r.Form.Get("db-number-new"), "db-number-new")
	case "", "existing":
		return parseRequiredInt64(r.Form.Get("db-number-existing"), "db-number-existing")
	default:
		return 0, fmt.Errorf("invalid db-number-mode")
	}
}

func fillStringForm(req *dbtool.CreateVarRequest, r *http.Request) error {
	strLen, err := parseRequiredInt64(r.Form.Get("str-length"), "str-length")
	if err != nil {
		return err
	}
	if strLen > math.MaxUint8 {
		return fmt.Errorf("string length cannot be greater than 255")
	}
	if strLen < 0 {
		return fmt.Errorf("string length cannot be negative")
	}
	if req.DataType == goprotos7.CHAR && strLen > 1 {
		return fmt.Errorf("CHAR data type cannot have length greater than 1")
	}
	l := uint8(strLen)
	req.StrLength = &l

	defVal := r.Form.Get("str-default-value")
	if defVal == "" {
		return fmt.Errorf("str-default-value not provided")
	}
	if req.DataType == goprotos7.CHAR && len(defVal) > 1 {
		return fmt.Errorf("CHAR default value cannot have length greater than 1")
	}
	req.StringVal = &defVal
	return nil
}

func fillCharForm(req *dbtool.CreateVarRequest, r *http.Request) error {
	defVal := r.Form.Get("char-default-value")
	if defVal == "" {
		return fmt.Errorf("char-default-value not provided")
	}
	if len(defVal) > 1 {
		return fmt.Errorf("CHAR default value cannot have length greater than 1")
	}
	req.StringVal = &defVal
	return nil
}

func fillBoolForm(req *dbtool.CreateVarRequest, r *http.Request) error {
	descriptions, ok := r.Form["desc-bool-field[]"]
	if !ok || len(descriptions) == 0 {
		return fmt.Errorf("desc-bool-field[] not provided")
	}
	boolVals, ok := r.Form["bool-value[]"]
	if !ok || len(boolVals) < len(descriptions) {
		return fmt.Errorf("bool-value[] not provided")
	}
	bitOffsets, ok := r.Form["bit-bool-field[]"]
	if !ok || len(bitOffsets) < len(descriptions) {
		return fmt.Errorf("bit-bool-field[] not provided")
	}

	req.ListFields = make([]*dbtool.ListFields, 0, len(descriptions))
	usedOffsets := map[int64]struct{}{}
	for i, desc := range descriptions {
		desc = strings.TrimSpace(desc)
		offsetRaw := strings.TrimSpace(bitOffsets[i])
		valueRaw := strings.TrimSpace(boolVals[i])
		if desc == "" && offsetRaw == "" && valueRaw == "" {
			continue
		}
		if desc == "" || offsetRaw == "" || valueRaw == "" {
			return fmt.Errorf("description, bit offset and default value are required for each bool entry")
		}
		offset, err := parseRequiredInt64(offsetRaw, "bit-bool-field[]")
		if err != nil {
			return err
		}
		if offset < 0 || offset > 7 {
			return fmt.Errorf("bit offset must be between 0 and 7")
		}
		if _, exists := usedOffsets[offset]; exists {
			return fmt.Errorf("duplicated bit offset: %d", offset)
		}
		usedOffsets[offset] = struct{}{}
		v := strings.ToUpper(valueRaw)
		b := v == "TRUE" || v == "1"
		req.ListFields = append(req.ListFields, &dbtool.ListFields{
			Description: desc,
			BoolValue:   &b,
			BitOffset:   &offset,
			StaticType:  dbtool.StaticTypeBool,
		})
	}
	if len(req.ListFields) == 0 {
		return fmt.Errorf("at least one bool entry is required")
	}
	return nil
}

func fillIntegerListForm(req *dbtool.CreateVarRequest, r *http.Request) error {
	rawDefault := r.Form.Get("default-int-value")
	if rawDefault == "" {
		return fmt.Errorf("default-int-value not provided")
	}
	defaultVal, err := strconv.ParseInt(rawDefault, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid default int value provided: %w", err)
	}
	req.IntVal = &defaultVal
	if err := validateIntegerRange(req.DataType, defaultVal); err != nil {
		return err
	}

	descriptions, hasDescriptions := r.Form["desc-int-field[]"]
	values, hasValues := r.Form["int-field[]"]
	if !hasValues || len(values) == 0 || !hasAnyNonEmpty(values) {
		return nil
	}
	if !hasDescriptions || len(descriptions) == 0 {
		return fmt.Errorf("desc-int-field[] not provided")
	}
	if len(values) != len(descriptions) {
		return fmt.Errorf("int-field[] length differs from desc-int-field[] length")
	}

	req.ListFields = make([]*dbtool.ListFields, 0, len(values))
	for idx, iv := range values {
		desc := strings.TrimSpace(descriptions[idx])
		rawVal := strings.TrimSpace(iv)
		if rawVal == "" && desc == "" {
			continue
		}
		if rawVal == "" || desc == "" {
			return fmt.Errorf("both description and value are required for each int list entry")
		}
		intVal, parseErr := strconv.ParseInt(rawVal, 10, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid int value provided: %w", parseErr)
		}
		if err := validateIntegerRange(req.DataType, intVal); err != nil {
			return err
		}
		req.ListFields = append(req.ListFields, &dbtool.ListFields{
			Description: desc,
			IntValue:    &intVal,
		})
	}
	return nil
}

func fillFloatListForm(req *dbtool.CreateVarRequest, r *http.Request) error {
	rawDefault := r.Form.Get("default-float-value")
	if rawDefault == "" {
		return fmt.Errorf("default-float-value not provided")
	}
	defaultVal, err := strconv.ParseFloat(rawDefault, 64)
	if err != nil {
		return fmt.Errorf("invalid default float value provided: %w", err)
	}
	req.FloatVal = &defaultVal

	descriptions, hasDescriptions := r.Form["desc-float-field[]"]
	values, hasValues := r.Form["float-field[]"]
	if !hasValues || len(values) == 0 || !hasAnyNonEmpty(values) {
		return nil
	}
	if !hasDescriptions || len(descriptions) == 0 {
		return fmt.Errorf("desc-float-field[] not provided")
	}
	if len(values) != len(descriptions) {
		return fmt.Errorf("float-field[] length differs from desc-float-field[] length")
	}

	req.ListFields = make([]*dbtool.ListFields, 0, len(values))
	for idx, iv := range values {
		desc := strings.TrimSpace(descriptions[idx])
		rawVal := strings.TrimSpace(iv)
		if rawVal == "" && desc == "" {
			continue
		}
		if rawVal == "" || desc == "" {
			return fmt.Errorf("both description and value are required for each float list entry")
		}
		floatVal, parseErr := strconv.ParseFloat(rawVal, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid float value provided: %w", parseErr)
		}
		req.ListFields = append(req.ListFields, &dbtool.ListFields{
			Description: desc,
			FloatValue:  &floatVal,
		})
	}
	return nil
}

func hasAnyNonEmpty(values []string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

func validateIntegerRange(dt goprotos7.DataType, value int64) error {
	switch dt {
	case goprotos7.BYTE, goprotos7.USINT:
		if value < 0 || value > math.MaxUint8 {
			return fmt.Errorf("%s value must be between 0 and %d", dt, math.MaxUint8)
		}
	case goprotos7.WORD, goprotos7.UINT:
		if value < 0 || value > math.MaxUint16 {
			return fmt.Errorf("%s value must be between 0 and %d", dt, math.MaxUint16)
		}
	case goprotos7.DWORD, goprotos7.UDINT:
		if value < 0 || value > math.MaxUint32 {
			return fmt.Errorf("%s value must be between 0 and %d", dt, math.MaxUint32)
		}
	case goprotos7.LWORD, goprotos7.ULINT:
		if value < 0 {
			return fmt.Errorf("%s cannot be negative", dt)
		}
	case goprotos7.SINT:
		if value < math.MinInt8 || value > math.MaxInt8 {
			return fmt.Errorf("SINT value must be between %d and %d", math.MinInt8, math.MaxInt8)
		}
	case goprotos7.INT:
		if value < math.MinInt16 || value > math.MaxInt16 {
			return fmt.Errorf("INT value must be between %d and %d", math.MinInt16, math.MaxInt16)
		}
	case goprotos7.DINT:
		if value < math.MinInt32 || value > math.MaxInt32 {
			return fmt.Errorf("DINT value must be between %d and %d", math.MinInt32, math.MaxInt32)
		}
	}
	return nil
}

func parseRequiredInt64(v string, field string) (int64, error) {
	if strings.TrimSpace(v) == "" {
		return 0, fmt.Errorf("%s is required", field)
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", field, err)
	}
	return n, nil
}
