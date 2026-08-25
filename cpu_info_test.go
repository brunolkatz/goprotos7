package goprotos7

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestUnpackUserDataSZLRequest(t *testing.T) {
	req := []byte{
		3, 0, 0, 33, 2, 240, 128, 50, 7, 0, 0,
		5, 0, 0, 8, 0, 8, 0, 1, 18, 4, 17, 68, 1, 0, 255, 9, 0, 4,
		0, 0x1C, 0, 0,
	}

	msg, err := unpack(req)
	if err != nil {
		t.Fatalf("unexpected unpack error: %v", err)
	}
	if msg.S7Request == nil {
		t.Fatal("S7 request is nil")
	}
	if msg.S7Request.FunctionCode != S7FuncUserData {
		t.Fatalf("unexpected function code: %x", msg.S7Request.FunctionCode)
	}

	userData, ok := msg.S7Request.FuncParam.(*S7ParamUserData)
	if !ok {
		t.Fatal("unexpected userdata param type")
	}
	if !userData.IsReadSZLRequest() {
		t.Fatal("expected Read SZL request")
	}

	szlID, szlIndex, err := userData.SZLIDIndex()
	if err != nil {
		t.Fatalf("unexpected SZL parse error: %v", err)
	}
	if szlID != 0x001C || szlIndex != 0x0000 {
		t.Fatalf("unexpected SZL id/index: %x/%x", szlID, szlIndex)
	}
}

func TestPackCPUInfoResponseLayout(t *testing.T) {
	reqMsg := &Message{
		TPKTHeader: TPKTHeader{Version: 3},
		COTPHeader: COTPHeader{Length: 2, PDUType: COTPData, EoT: 0x80},
		S7Header: &S7Header{
			ProtocolID:       S7ProtocolID,
			ROSCTR:           S7FuncUserData,
			RedundancyID:     0,
			ProtocolDataUnit: 0x0500,
			ParamLength:      8,
			DataLength:       8,
		},
		S7Request: &S7Request{
			FunctionCode: S7FuncUserData,
			FuncParam: &S7ParamUserData{
				Parameter: []byte{0x00, 0x01, 0x12, 0x04, 0x11, 0x44, 0x01, 0x00},
				Data:      []byte{0xFF, 0x09, 0x00, 0x04, 0x00, 0x1C, 0x00, 0x00},
			},
		},
	}

	res := buildCPUInfoResponseMessage(reqMsg, 0x00, 0x001C, 0x0000)
	packed, err := res.Pack(COTPData)
	if err != nil {
		t.Fatalf("unexpected pack error: %v", err)
	}

	if packed[24] != 0x00 {
		t.Fatalf("unexpected sequence byte: got %x", packed[24])
	}
	if packed[26] != 0x00 {
		t.Fatalf("expected last-data-unit byte at [26] == 0, got %x", packed[26])
	}

	dataSZL := int(binary.BigEndian.Uint16(packed[31:33])) - 8
	if dataSZL < 204 {
		t.Fatalf("expected dataSZL >= 204, got %d", dataSZL)
	}

	szlData := packed[41 : 41+dataSZL]
	asName := strings.TrimSpace(string(szlData[2 : 2+24]))
	moduleName := strings.TrimSpace(string(szlData[36 : 36+24]))
	copyRight := strings.TrimSpace(string(szlData[104 : 104+26]))
	serialNumber := strings.TrimSpace(string(szlData[138 : 138+24]))
	moduleTypeName := strings.TrimSpace(string(szlData[172 : 172+32]))

	if asName != defaultS7CPUInfo.ASName {
		t.Fatalf("unexpected ASName: %q", asName)
	}
	if moduleName != defaultS7CPUInfo.ModuleName {
		t.Fatalf("unexpected ModuleName: %q", moduleName)
	}
	if copyRight != defaultS7CPUInfo.Copyright {
		t.Fatalf("unexpected Copyright: %q", copyRight)
	}
	if serialNumber != defaultS7CPUInfo.SerialNumber {
		t.Fatalf("unexpected SerialNumber: %q", serialNumber)
	}
	if moduleTypeName != defaultS7CPUInfo.ModuleTypeName {
		t.Fatalf("unexpected ModuleTypeName: %q", moduleTypeName)
	}
}
