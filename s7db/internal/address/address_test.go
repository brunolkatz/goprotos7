package address

import "testing"

func intp(v int) *int { return &v }

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		defaultDB *int
		want      Address
		wantErr   bool
	}{
		{
			name: "full bool",
			in:   "DB10.DBX0.3",
			want: Address{DB: 10, Area: "X", Byte: 0, Bit: 3},
		},
		{
			name: "full word",
			in:   "DB10.DBW2",
			want: Address{DB: 10, Area: "W", Byte: 2},
		},
		{
			name:      "short with default db",
			in:        "D4",
			defaultDB: intp(10),
			want:      Address{DB: 10, Area: "D", Byte: 4},
		},
		{
			name:      "short bool with default db",
			in:        "X1.7",
			defaultDB: intp(10),
			want:      Address{DB: 10, Area: "X", Byte: 1, Bit: 7},
		},
		{
			name:    "short without db fails",
			in:      "W2",
			wantErr: true,
		},
		{
			name:    "bool bit required",
			in:      "DB10.DBX1",
			wantErr: true,
		},
		{
			name:    "bit on non bool fails",
			in:      "DB10.DBW2.1",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.in, tc.defaultDB)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none (%+v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
