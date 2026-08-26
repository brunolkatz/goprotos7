package create_var_api

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/brunolkatz/goprotos7"
	"github.com/brunolkatz/goprotos7/dbtool"
	"github.com/brunolkatz/goprotos7/dbtool/api/httpx"
)

//go:embed examples/db_variables_import_example.csv
var importCSVExampleFile []byte

func (h *CreateVarAPi) DownloadImportCSVExample(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="db_variables_import_example.csv"`)
	_, _ = w.Write(importCSVExampleFile)
}

func (h *CreateVarAPi) ImportCSVVariables(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("csv-file")
	if err != nil {
		httpx.AlertError(w, r, "Error reading csv file: "+err.Error())
		return
	}
	defer file.Close()

	rows, err := parseCSVImportFile(file)
	if err != nil {
		httpx.AlertError(w, r, "Error parsing csv: "+err.Error())
		return
	}
	if len(rows) == 0 {
		httpx.AlertError(w, r, "CSV file has no variable rows")
		return
	}

	created := 0
	for idx, row := range rows {
		if _, err := h.varsHandler.CreateVariable(r.Context(), row); err != nil {
			httpx.AlertError(w, r, fmt.Sprintf("Error on row %d (%s): %s", idx+2, row.Name, err.Error()))
			return
		}
		created++
	}
	httpx.AlertSuccess(w, r, fmt.Sprintf("Imported %d variable(s) successfully", created))
}

func parseCSVImportFile(reader io.Reader) ([]*dbtool.CreateVarRequest, error) {
	csvReader := csv.NewReader(reader)
	csvReader.Comma = ';'
	csvReader.TrimLeadingSpace = true

	header, err := csvReader.Read()
	if err != nil {
		return nil, err
	}
	indexByName := make(map[string]int, len(header))
	for i, h := range header {
		indexByName[strings.ToLower(strings.TrimSpace(h))] = i
	}
	required := []string{"db_number", "name", "description", "data_type"}
	for _, r := range required {
		if _, ok := indexByName[r]; !ok {
			return nil, fmt.Errorf("missing required column: %s", r)
		}
	}

	out := make([]*dbtool.CreateVarRequest, 0)
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) == 0 {
			continue
		}
		if rowIsEmpty(record) {
			continue
		}
		req, err := parseCSVRecord(indexByName, record)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, nil
}

func parseCSVRecord(index map[string]int, record []string) (*dbtool.CreateVarRequest, error) {
	get := func(key string) string {
		i, ok := index[key]
		if !ok || i < 0 || i >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[i])
	}

	dbNumber, err := strconv.ParseInt(get("db_number"), 10, 64)
	if err != nil || dbNumber <= 0 {
		return nil, fmt.Errorf("invalid db_number: %s", get("db_number"))
	}
	name := get("name")
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	description := get("description")
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	dataType, err := parseCSVDataType(get("data_type"))
	if err != nil {
		return nil, err
	}
	req := &dbtool.CreateVarRequest{
		DBNumber:    dbNumber,
		Name:        name,
		Description: description,
		DataType:    dataType,
	}

	defaultValue := get("default_value")
	strLengthRaw := get("str_length")
	presetsRaw := get("presets")
	boolBitsRaw := get("bool_bits")

	switch dataType {
	case goprotos7.BOOL:
		list, err := parseBoolBits(boolBitsRaw)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			return nil, fmt.Errorf("bool_bits is required for BOOL")
		}
		req.ListFields = list
	case goprotos7.STRING:
		if defaultValue == "" {
			return nil, fmt.Errorf("default_value is required for STRING")
		}
		l, err := strconv.ParseInt(strLengthRaw, 10, 64)
		if err != nil || l <= 0 || l > 255 {
			return nil, fmt.Errorf("str_length must be between 1 and 255 for STRING")
		}
		u := uint8(l)
		req.StrLength = &u
		req.StringVal = &defaultValue
	case goprotos7.CHAR:
		if defaultValue == "" || len(defaultValue) > 1 {
			return nil, fmt.Errorf("default_value for CHAR must be one character")
		}
		req.StringVal = &defaultValue
	case goprotos7.REAL, goprotos7.LREAL:
		fv, err := strconv.ParseFloat(defaultValue, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid default_value for float type")
		}
		req.FloatVal = &fv
		list, err := parseFloatPresets(presetsRaw)
		if err != nil {
			return nil, err
		}
		req.ListFields = list
	default:
		iv, err := strconv.ParseInt(defaultValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid default_value for integer type")
		}
		req.IntVal = &iv
		list, err := parseIntPresets(presetsRaw)
		if err != nil {
			return nil, err
		}
		req.ListFields = list
	}
	return req, nil
}

func parseCSVDataType(raw string) (goprotos7.DataType, error) {
	if raw == "" {
		return 0, fmt.Errorf("data_type is required")
	}
	if n, err := strconv.ParseUint(raw, 10, 32); err == nil {
		return goprotos7.DataType(n), nil
	}
	target := strings.ToUpper(strings.TrimSpace(raw))
	for _, dt := range goprotos7.OrderedDataTypes {
		if strings.EqualFold(dt.String(), target) {
			return dt, nil
		}
	}
	return 0, fmt.Errorf("invalid data_type: %s", raw)
}

func parseIntPresets(raw string) ([]*dbtool.ListFields, error) {
	items := splitPresetEntries(raw)
	out := make([]*dbtool.ListFields, 0, len(items))
	for _, item := range items {
		label, valueRaw, err := splitLabelValue(item)
		if err != nil {
			return nil, err
		}
		v, err := strconv.ParseInt(valueRaw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid int preset value: %s", valueRaw)
		}
		out = append(out, &dbtool.ListFields{Description: label, IntValue: &v})
	}
	return out, nil
}

func parseFloatPresets(raw string) ([]*dbtool.ListFields, error) {
	items := splitPresetEntries(raw)
	out := make([]*dbtool.ListFields, 0, len(items))
	for _, item := range items {
		label, valueRaw, err := splitLabelValue(item)
		if err != nil {
			return nil, err
		}
		v, err := strconv.ParseFloat(valueRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float preset value: %s", valueRaw)
		}
		out = append(out, &dbtool.ListFields{Description: label, FloatValue: &v})
	}
	return out, nil
}

func parseBoolBits(raw string) ([]*dbtool.ListFields, error) {
	items := splitPresetEntries(raw)
	out := make([]*dbtool.ListFields, 0, len(items))
	used := map[int64]struct{}{}
	for _, item := range items {
		parts := strings.Split(item, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid bool_bits entry: %s", item)
		}
		desc := strings.TrimSpace(parts[0])
		offset, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil || offset < 0 || offset > 7 {
			return nil, fmt.Errorf("invalid BOOL bit offset in %s", item)
		}
		if _, ok := used[offset]; ok {
			return nil, fmt.Errorf("duplicated BOOL bit offset: %d", offset)
		}
		used[offset] = struct{}{}
		b, err := strconv.ParseBool(strings.TrimSpace(parts[2]))
		if err != nil {
			return nil, fmt.Errorf("invalid BOOL default value in %s", item)
		}
		out = append(out, &dbtool.ListFields{
			Description: desc,
			BitOffset:   &offset,
			BoolValue:   &b,
			StaticType:  dbtool.StaticTypeBool,
		})
	}
	return out, nil
}

func splitPresetEntries(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	chunks := strings.Split(raw, "|")
	out := make([]string, 0, len(chunks))
	for _, c := range chunks {
		t := strings.TrimSpace(c)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func splitLabelValue(raw string) (string, string, error) {
	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid preset entry: %s", raw)
	}
	label := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if label == "" || value == "" {
		return "", "", fmt.Errorf("invalid preset entry: %s", raw)
	}
	return label, value, nil
}

func rowIsEmpty(record []string) bool {
	for _, v := range record {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}
