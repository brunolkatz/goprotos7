package create_var_api

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBindCreateVarRequestREALWithPresets(t *testing.T) {
	form := url.Values{}
	form.Set("db-number-mode", "new")
	form.Set("db-number-new", "200")
	form.Set("name", "Temperature")
	form.Set("description", "Process temperature")
	form.Set("data_type", "13") // REAL
	form.Set("default-float-value", "20.5")
	form.Add("desc-float-field[]", "LOW")
	form.Add("float-field[]", "15.0")
	form.Add("desc-float-field[]", "WORK")
	form.Add("float-field[]", "20.5")

	req := httptest.NewRequest("POST", "/vars/create-var", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	got, err := bindCreateVarRequest(req)
	if err != nil {
		t.Fatalf("unexpected bind error: %v", err)
	}

	if got == nil || got.FloatVal == nil {
		t.Fatalf("expected float request payload")
	}
	if *got.FloatVal != 20.5 {
		t.Fatalf("unexpected default float value: %v", *got.FloatVal)
	}
	if len(got.ListFields) != 2 {
		t.Fatalf("expected 2 float presets, got %d", len(got.ListFields))
	}
	if got.ListFields[0].FloatValue == nil || *got.ListFields[0].FloatValue != 15.0 {
		t.Fatalf("unexpected first float preset")
	}
}
