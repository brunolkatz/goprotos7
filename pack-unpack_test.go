package goprotos7

import (
	"reflect"
	"testing"
)

func Test_unpack(t *testing.T) {
	type args struct {
		b []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *Message
		wantErr bool
	}{
		{
			name: "1 - unpack read data request",
			args: args{
				b: []byte{0x03, 0x00, 0x00, 0x1F, 0x02, 0xF0, 0x80, 0x32, 0x01, 0x00, 0x00, 0x05, 0x00, 0x00, 0x0E, 0x00, 0x00, 0x04, 0x01, 0x12, 0x0A, 0x10, 0x02, 0x00, 0x02, 0x00, 0xC8, 0x84, 0x00, 0x0E, 0xA0},
			},
			want: &Message{
				TPKTHeader: TPKTHeader{
					Version:  3,
					Reserved: 0,
					Length:   31,
				},
				COTPHeader: COTPHeader{
					Length:  0x02,
					PDUType: 0xF0,
					EoT:     0x80,
				},
				S7Header: &S7Header{
					ProtocolID:   S7ProtocolID,
					ROSCTR:       0x01,
					RedundancyId: 0,
					ParamLength:  1280,
					DataLength:   14,
					ErrorClass:   0,
					ErrorCode:    0,
				},
				S7Request: nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unpack(tt.args.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("unpack() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unpack() got = %v, want %v", got, tt.want)
			}
		})
	}
}
