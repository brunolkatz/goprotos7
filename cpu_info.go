package goprotos7

import (
	"encoding/binary"
	"log"
	"net"
)

type S7CpuInfo struct {
	ModuleTypeName string
	SerialNumber   string
	ASName         string
	Copyright      string
	ModuleName     string
}

var defaultS7CPUInfo = S7CpuInfo{
	ModuleTypeName: "CPU 315-2 PN/DP",
	SerialNumber:   "S7EMU000000000000000001",
	ASName:         "GOPROTOS7",
	Copyright:      "(C) goprotos7",
	ModuleName:     "GOPROTOS7 CPU",
}

func (c *Connection) eventS7FuncUserData(msg *Message, conn net.Conn) {
	if msg == nil || msg.S7Request == nil || msg.S7Request.FuncParam == nil {
		return
	}

	userData, ok := msg.S7Request.FuncParam.(*S7ParamUserData)
	if !ok {
		return
	}
	if !userData.IsReadSZLRequest() {
		return
	}

	szlID, szlIndex, err := userData.SZLIDIndex()
	if err != nil {
		log.Printf("[SERVER] Error parsing SZL request: %s", err)
		return
	}

	if szlID != 0x001C || szlIndex != 0x0000 {
		return
	}

	res := buildCPUInfoResponseMessage(msg, userData.Sequence(), szlID, szlIndex)
	ack, err := res.Pack(COTPData)
	if err != nil {
		log.Printf("[SERVER] Error packing CPU info response: %s", err)
		return
	}
	_, err = conn.Write(ack)
	if err != nil {
		log.Printf("[SERVER] Error writing CPU info response: %s", err)
	}
}

func buildCPUInfoResponseMessage(msg *Message, sequence byte, szlID uint16, szlIndex uint16) *Message {
	payload := buildCPUInfoSZLPayload(defaultS7CPUInfo)

	param := []byte{0x00, 0x01, 0x12, 0x04, 0x11, 0x44, 0x01, sequence}
	if userDataReq, ok := msg.S7Request.FuncParam.(*S7ParamUserData); ok && len(userDataReq.Parameter) == 8 {
		copy(param, userDataReq.Parameter)
		param[7] = sequence
	}

	data := make([]byte, 16+len(payload))
	data[0] = 0xFF // Return code
	data[1] = 0x00 // Last data unit
	binary.BigEndian.PutUint16(data[2:4], 0x0000)
	data[4] = 0xFF
	data[5] = 0x09
	binary.BigEndian.PutUint16(data[6:8], uint16(len(payload)+8))
	binary.BigEndian.PutUint16(data[8:10], szlID)
	binary.BigEndian.PutUint16(data[10:12], szlIndex)
	binary.BigEndian.PutUint16(data[12:14], 0x001C) // LengthHeader
	binary.BigEndian.PutUint16(data[14:16], 0x0007) // NumberOfDataRecord
	copy(data[16:], payload)

	return &Message{
		TPKTHeader: msg.TPKTHeader,
		COTPHeader: msg.COTPHeader,
		S7Header: &S7Header{
			ProtocolID:       S7ProtocolID,
			ROSCTR:           S7FuncUserData,
			RedundancyID:     msg.S7Header.RedundancyID,
			ProtocolDataUnit: msg.S7Header.ProtocolDataUnit,
			ParamLength:      uint16(len(param)),
			DataLength:       uint16(len(data)),
		},
		S7Response: &S7Response{
			FunctionCode: S7FuncUserData,
			FuncParam: &S7ParamUserData{
				Parameter: param,
				Data:      data,
			},
		},
	}
}

func buildCPUInfoSZLPayload(info S7CpuInfo) []byte {
	payload := make([]byte, 204)
	for i := range payload {
		payload[i] = ' '
	}

	writeFixedString(payload, 2, 24, info.ASName)
	writeFixedString(payload, 36, 24, info.ModuleName)
	writeFixedString(payload, 104, 26, info.Copyright)
	writeFixedString(payload, 138, 24, info.SerialNumber)
	writeFixedString(payload, 172, 32, info.ModuleTypeName)

	return payload
}

func writeFixedString(dst []byte, offset int, size int, value string) {
	if offset >= len(dst) || size <= 0 {
		return
	}
	end := offset + size
	if end > len(dst) {
		end = len(dst)
	}
	copy(dst[offset:end], []byte(value))
}
