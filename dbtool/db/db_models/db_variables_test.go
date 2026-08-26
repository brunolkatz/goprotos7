package db_models

import (
	"github.com/brunolkatz/goprotos7"
	"github.com/brunolkatz/goprotos7/dbtool"
	"testing"
)

func TestUpdateIsSelectedFloat(t *testing.T) {
	current := 42.5
	presetA := 10.1
	presetB := 42.5

	v := &DbVariable{
		DataType:   goprotos7.REAL,
		FloatVal:   &current,
		VarType:    dbtool.VarTypeList,
		ByteOffset: 12,
		StaticVarDefinitions: []*StaticVarDefinition{
			{Id: 1, Description: "LOW", FloatValue: &presetA, StaticType: dbtool.StaticTypeFloat},
			{Id: 2, Description: "WORK", FloatValue: &presetB, StaticType: dbtool.StaticTypeFloat},
		},
	}

	v.UpdateIsSelected()
	if !v.StaticVarDefinitions[1].IsSelected {
		t.Fatalf("expected float preset to be selected")
	}
	if v.StaticVarDefinitions[0].IsSelected {
		t.Fatalf("unexpected non-matching float preset selected")
	}
}

func TestToDBAddressREALUsesByteOffset(t *testing.T) {
	v := &DbVariable{
		DbNumber:   200,
		DataType:   goprotos7.REAL,
		ByteOffset: 24,
	}
	got := v.ToDBAddress()
	want := "DB200.DBD24"
	if got != want {
		t.Fatalf("unexpected real address: got %q want %q", got, want)
	}
}
