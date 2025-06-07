package build_datab

import (
	"github.com/brunolkatz/goprotos7"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func getIntPtr(v int64) *int64 {
	return &v
}

func getUint8Ptr(v uint8) *uint8 {
	return &v
}

func getBoolPtr(v bool) *bool {
	return &v
}

func getStrPtr(v string) *string {
	return &v
}

func openFile(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		panic(err)
	}
	b := make([]byte, fi.Size())
	_, err = f.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func Test_createBuffer(t *testing.T) {
	type args struct {
		variables []*db_models.DbVariable
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "1 - Test BOOL + STRING[10]",
			args: args{
				variables: []*db_models.DbVariable{
					&db_models.DbVariable{
						DataType:   goprotos7.BOOL,
						ByteOffset: 0,
						BitOffset:  getIntPtr(0),
						BoolVal:    getBoolPtr(true),
					},
					&db_models.DbVariable{
						DataType:   goprotos7.STRING,
						ByteOffset: 1,
						Length:     getUint8Ptr(10),
					},
				},
			},
			want:    []byte{0x00000001, 0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantErr: false,
		},
		{
			name: "2 - Test BOOL + BOOL + STRING[10]",
			args: args{
				variables: []*db_models.DbVariable{
					&db_models.DbVariable{
						DataType:   goprotos7.BOOL,
						ByteOffset: 0,
						BitOffset:  getIntPtr(0),
						BoolVal:    getBoolPtr(true),
					},
					&db_models.DbVariable{
						DataType:   goprotos7.BOOL,
						ByteOffset: 0,
						BitOffset:  getIntPtr(1),
						BoolVal:    getBoolPtr(true),
					},
					&db_models.DbVariable{
						DataType:   goprotos7.STRING,
						ByteOffset: 1,
						Length:     getUint8Ptr(10),
					},
				},
			},
			want:    []byte{0x3, 0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantErr: false,
		},
		{
			name: "3 - STRING[10]",
			args: args{
				variables: []*db_models.DbVariable{
					&db_models.DbVariable{
						DataType:   goprotos7.STRING,
						ByteOffset: 0,
						Length:     getUint8Ptr(11),
						StringVal:  getStrPtr("hello world"),
					},
				},
			},
			want:    []byte{0xb, 0xb, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64},
			wantErr: false,
		},
		{
			name: "2 - Test BOOL + BOOL + STRING[10] +  BOOL",
			args: args{
				variables: []*db_models.DbVariable{
					&db_models.DbVariable{
						DataType:   goprotos7.BOOL,
						ByteOffset: 0,
						BitOffset:  getIntPtr(0),
						BoolVal:    getBoolPtr(true),
					},
					&db_models.DbVariable{
						DataType:   goprotos7.BOOL,
						ByteOffset: 0,
						BitOffset:  getIntPtr(1),
						BoolVal:    getBoolPtr(true),
					},
					&db_models.DbVariable{
						DataType:   goprotos7.STRING,
						ByteOffset: 1,
						Length:     getUint8Ptr(11),
						StringVal:  getStrPtr("hello world"),
					},
					&db_models.DbVariable{
						DataType:   goprotos7.BOOL,
						ByteOffset: 14,
						BitOffset:  getIntPtr(1),
						BoolVal:    getBoolPtr(true),
					},
				},
			},
			want:    []byte{0x3, 0xb, 0xb, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64, 0x2},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createBuffer(tt.args.variables)
			if (err != nil) != tt.wantErr {
				t.Errorf("createBuffer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createBuffer() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildDataBlocks(t *testing.T) {

	// get the pwd
	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("os.Getwd() error = %v", err)
		return
	}

	type args struct {
		path      string
		variables []*db_models.DbVariable
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		wantBuf []byte
	}{
		{
			name: "1 - Create file",
			args: args{
				path: filepath.Join(pwd, "testdata.bin"),
				variables: []*db_models.DbVariable{
					{
						DataType:   goprotos7.BOOL,
						ByteOffset: 0,
						BitOffset:  getIntPtr(0),
						BoolVal:    getBoolPtr(true),
					},
					{
						DataType:   goprotos7.STRING,
						ByteOffset: 1,
						Length:     getUint8Ptr(10),
					},
				},
			},
			wantErr: false,
			wantBuf: []byte{0x00000001, 0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := BuildDataBlocks(tt.args.path, tt.args.variables); (err != nil) != tt.wantErr {
				t.Errorf("BuildDataBlocks() error = %v, wantErr %v", err, tt.wantErr)
			}
			buf := openFile(tt.args.path)
			if !reflect.DeepEqual(buf, tt.wantBuf) {
				t.Errorf("BuildDataBlocks() buf = %v, wantBuf %v", buf, tt.wantBuf)
			}
			// delete the created file
			err = os.Remove(tt.args.path)
			if err != nil {
				t.Errorf("os.Remove() error = %v", err)
			}
			return
		})
	}
}

func Test_writeToFile(t *testing.T) {

	// get the pwd
	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("os.Getwd() error = %v", err)
		return
	}

	type args struct {
		path       string
		dbVariable *db_models.DbVariable
	}
	tests := []struct {
		name            string
		args            args
		variables2Write []*db_models.DbVariable
		wantBuf         []byte // To verify the content of the file before writing
		wantAfterBuf    []byte // To verify the content of the file after writing
		wantErr         bool
	}{
		{
			name: "1 - write BOOL and STRING[10] to file check and change, check again",
			args: args{
				path: filepath.Join(pwd, "testdata.bin"),
				dbVariable: &db_models.DbVariable{
					DataType:   goprotos7.STRING,
					ByteOffset: 1,
					StringVal:  getStrPtr("olleh"),
					Length:     getUint8Ptr(5),
				},
			},
			variables2Write: []*db_models.DbVariable{
				{
					DataType:   goprotos7.BOOL,
					ByteOffset: 0,
					BitOffset:  getIntPtr(0),
					BoolVal:    getBoolPtr(true),
				},
				{
					DataType:   goprotos7.BOOL,
					ByteOffset: 0,
					BitOffset:  getIntPtr(1),
					BoolVal:    getBoolPtr(true),
				},
				{
					DataType:   goprotos7.STRING,
					ByteOffset: 1,
					StringVal:  getStrPtr("hello"),
					Length:     getUint8Ptr(5),
				},
			},
			wantBuf:      []byte{0x3, 5, 5, 0x68, 0x65, 0x6C, 0x6C, 0x6F},
			wantAfterBuf: []byte{0x3, 5, 5, 0x6F, 0x6C, 0x6C, 0x65, 0x68},
			wantErr:      false,
		},
		{
			name: "1 - write BOOL and STRING[10] to file check and change, check again",
			args: args{
				path: filepath.Join(pwd, "testdata.bin"),
				dbVariable: &db_models.DbVariable{
					DataType:   goprotos7.BOOL,
					ByteOffset: 0,
					BitOffset:  getIntPtr(1),
					BoolVal:    getBoolPtr(true),
				},
			},
			variables2Write: []*db_models.DbVariable{
				{
					DataType:   goprotos7.BOOL,
					ByteOffset: 0,
					BitOffset:  getIntPtr(0),
					BoolVal:    getBoolPtr(true),
				},
				{
					DataType:   goprotos7.BOOL,
					ByteOffset: 0,
					BitOffset:  getIntPtr(1),
					BoolVal:    getBoolPtr(false),
				},
				{
					DataType:   goprotos7.STRING,
					ByteOffset: 1,
					StringVal:  getStrPtr("hello"),
					Length:     getUint8Ptr(5),
				},
			},
			wantBuf:      []byte{0x1, 5, 5, 0x68, 0x65, 0x6C, 0x6C, 0x6F},
			wantAfterBuf: []byte{0x3, 5, 5, 0x68, 0x65, 0x6C, 0x6C, 0x6F},
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First Write to initial variables
			if err := BuildDataBlocks(tt.args.path, tt.variables2Write); (err != nil) != tt.wantErr {
				t.Errorf("BuildDataBlocks() error = %v, wantErr %v", err, tt.wantErr)
			}
			// check the content of the file
			buf := openFile(tt.args.path)
			if !reflect.DeepEqual(buf, tt.wantBuf) {
				t.Errorf("BuildDataBlocks() buf = %v, wantBuf %v", buf, tt.wantBuf)
			}

			// Write the new variable to the file
			if _, err := writeToFile(tt.args.path, tt.args.dbVariable); (err != nil) != tt.wantErr {
				t.Errorf("writeToFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			// check the content of the file after writing
			buf = openFile(tt.args.path)
			if !reflect.DeepEqual(buf, tt.wantAfterBuf) {
				t.Errorf("writeToFile() buf = %v, wantAfterBuf %v", buf, tt.wantAfterBuf)
			}

			// delete the created file
			err = os.Remove(tt.args.path)
			if err != nil {
				t.Errorf("os.Remove() error = %v", err)
			}
			return
		})
	}
}
