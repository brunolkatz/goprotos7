package create_var_api

import (
	"strings"
	"testing"

	"github.com/brunolkatz/goprotos7"
)

func TestParseCSVImportFile_WithPresetsAndBoolBits(t *testing.T) {
	csvText := `db_number;name;description;data_type;default_value;str_length;presets;bool_bits
203;Temp;Process temp;REAL;20.5;;LOW=15.0|WORK=20.5;
203;Flags;Status bits;BOOL;;;;Ready:0:true|Error:1:false
`
	rows, err := parseCSVImportFile(strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].DataType != goprotos7.REAL || rows[0].FloatVal == nil {
		t.Fatalf("expected REAL row with float default value")
	}
	if len(rows[0].ListFields) != 2 {
		t.Fatalf("expected 2 presets for REAL row, got %d", len(rows[0].ListFields))
	}
	if rows[1].DataType != goprotos7.BOOL {
		t.Fatalf("expected BOOL second row")
	}
	if len(rows[1].ListFields) != 2 {
		t.Fatalf("expected 2 bool bits, got %d", len(rows[1].ListFields))
	}
}
