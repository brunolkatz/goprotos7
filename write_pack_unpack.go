package goprotos7

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type S7WriteVarDataItem struct {
	ReturnCode    byte
	TransportSize byte
	Length        uint16 // Bit length for BYTE/WORD/DWORD payloads
	Data          []byte
}

type S7ParamWriteVar struct {
	Items     []S7VarRequestItem
	DataItems []S7WriteVarDataItem
}

func (s S7ParamWriteVar) Pack() []byte {
	ret := make([]byte, 0)
	ret = append(ret, byte(len(s.Items)))
	for _, item := range s.Items {
		ret = append(ret, item.VarSpec)
		ret = append(ret, item.LenAddrSpec)
		ret = append(ret, byte(item.SyntaxID))
		ret = append(ret, item.TransportSize)
		ret = append(ret, byte(item.Length>>8), byte(item.Length&0xFF))
		ret = append(ret, byte(item.DBNumber>>8), byte(item.DBNumber&0xFF))
		ret = append(ret, item.Area)
		ret = append(ret, byte(item.Address>>16), byte(item.Address>>8), byte(item.Address&0xFF))
	}
	for _, d := range s.DataItems {
		ret = append(ret, d.ReturnCode)
		ret = append(ret, d.TransportSize)
		ret = append(ret, byte(d.Length>>8), byte(d.Length&0xFF))
		ret = append(ret, d.Data...)
	}
	return ret
}

type S7ResponseWriteVar struct {
	ItemCount byte
	Results   []S7IItemReturnCode
}

func (s S7ResponseWriteVar) Pack() []byte {
	ret := make([]byte, 0, 1+len(s.Results))
	ret = append(ret, s.ItemCount)
	for _, code := range s.Results {
		ret = append(ret, byte(code))
	}
	return ret
}

// unpackWriteVarPacket parses WriteVar packets without touching the main read/userdata unpack flow.
func unpackWriteVarPacket(b []byte) (*Message, error) {
	if len(b) < 4 {
		return nil, errors.New("TPKT header is too short")
	}

	ret := Message{
		TPKTHeader: TPKTHeader{
			Version:  b[0],
			Reserved: b[1],
			Length:   uint16(b[2])<<8 | uint16(b[3]),
		},
	}
	if len(b) < int(ret.TPKTHeader.Length) {
		return nil, errors.New("TPKT header length is too short")
	}

	if len(b) < 7 {
		return nil, errors.New("COTP header is too short")
	}
	ret.COTPHeader.Length = b[4]
	ret.COTPHeader.PDUType = b[5]
	if ret.COTPHeader.PDUType != COTPData {
		return nil, ErrorFunctionCodeNotSupported
	}
	ret.COTPHeader.EoT = b[6]

	if len(b) < 18 || b[7] != S7ProtocolID {
		return nil, errors.New("the S7 header is not valid")
	}
	s7B := b[7:]
	s7 := &S7Header{
		ProtocolID:       s7B[0],
		ROSCTR:           s7B[1],
		RedundancyID:     binary.BigEndian.Uint16(s7B[2:4]),
		ProtocolDataUnit: binary.BigEndian.Uint16(s7B[4:6]),
		ParamLength:      binary.BigEndian.Uint16(s7B[6:8]),
		DataLength:       binary.BigEndian.Uint16(s7B[8:10]),
	}
	payloadOffset := 10
	if s7.ROSCTR == S7FuncAck || s7.ROSCTR == S7FuncAckData {
		if len(s7B) < 12 {
			return nil, errors.New("S7 header with ACK is too short")
		}
		s7.ErrorClass = s7B[10]
		s7.ErrorCode = s7B[11]
		payloadOffset = 12
	}

	if len(s7B[payloadOffset:]) < 1 {
		return nil, errors.New("S7 payload is too short")
	}
	functionCode := s7B[payloadOffset]
	if functionCode != S7FuncWriteVar {
		return nil, ErrorFunctionCodeNotSupported
	}

	payload := s7B[payloadOffset:]
	if len(payload) < int(s7.ParamLength)+int(s7.DataLength) {
		return nil, fmt.Errorf("S7 WriteVar payload too short")
	}
	if s7.ParamLength < 2 {
		return nil, fmt.Errorf("S7 WriteVar param length too short")
	}
	if uint16(len(payload)) < s7.ParamLength {
		return nil, fmt.Errorf("S7 WriteVar param data is too short")
	}

	paramData := payload[1:s7.ParamLength]
	if len(paramData) < 1 {
		return nil, fmt.Errorf("S7 WriteVar item count missing")
	}
	itemsCount := int(paramData[0])
	pos := 1
	items := make([]S7VarRequestItem, 0, itemsCount)
	for i := 0; i < itemsCount; i++ {
		if len(paramData[pos:]) < 12 {
			return nil, fmt.Errorf("not enough data to parse write item %d", i)
		}
		item := S7VarRequestItem{
			VarSpec:     paramData[pos],
			LenAddrSpec: paramData[pos+1],
		}
		if item.LenAddrSpec != 0x0A {
			return nil, fmt.Errorf("unexpected write item address spec length: %d", item.LenAddrSpec)
		}
		addressSpec := paramData[pos+2 : pos+12]
		item.SyntaxID = SyntaxID(addressSpec[0])
		item.TransportSize = addressSpec[1]
		item.Length = binary.BigEndian.Uint16(addressSpec[2:4])
		item.DBNumber = binary.BigEndian.Uint16(addressSpec[4:6])
		item.Area = addressSpec[6]
		item.Address = uint32(addressSpec[7])<<16 | uint32(addressSpec[8])<<8 | uint32(addressSpec[9])
		item.ByteOffset = item.Address / 8
		item.BitOffset = byte(item.Address % 8)
		items = append(items, item)
		pos += 12
	}

	data := payload[s7.ParamLength : s7.ParamLength+s7.DataLength]
	dataPos := 0
	dataItems := make([]S7WriteVarDataItem, 0, itemsCount)
	for i := 0; i < itemsCount; i++ {
		if len(data[dataPos:]) < 4 {
			return nil, fmt.Errorf("not enough data to parse write data item %d", i)
		}
		returnCode := data[dataPos]
		transportSize := data[dataPos+1]
		bitLength := binary.BigEndian.Uint16(data[dataPos+2 : dataPos+4])
		dataPos += 4

		byteLength := payloadByteLength(transportSize, bitLength)
		if len(data[dataPos:]) < byteLength {
			return nil, fmt.Errorf("write payload item %d is truncated", i)
		}
		itemData := append([]byte(nil), data[dataPos:dataPos+byteLength]...)
		dataPos += byteLength
		dataItems = append(dataItems, S7WriteVarDataItem{
			ReturnCode:    returnCode,
			TransportSize: transportSize,
			Length:        bitLength,
			Data:          itemData,
		})
	}

	ret.S7Header = s7
	ret.S7Request = &S7Request{
		FunctionCode: functionCode,
		DataSection:  s7B[payloadOffset+1:],
		FuncParam: &S7ParamWriteVar{
			Items:     items,
			DataItems: dataItems,
		},
	}
	return &ret, nil
}

func payloadByteLength(transportSize byte, bitLength uint16) int {
	switch transportSize {
	case 0x03: // BIT
		return 1
	default:
		// WriteVar bit length is usually expressed in bits for BYTE/WORD/... payloads.
		return int((bitLength + 7) / 8)
	}
}
