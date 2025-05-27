package goprotos7

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"reflect"
)

type VariableType int

const (
	TRANSPORT_SIZE_BOOL          VariableType = 0x01 // 1 bit (BOOL)
	TRANSPORT_SIZE_BYTE          VariableType = 0x02 // 1 byte (BYTE)
	TRANSPORT_SIZE_WORD          VariableType = 0x04 // 2 bytes (WORD)
	TRANSPORT_SIZE_DWORD         VariableType = 0x06 // 4 bytes (DWORD)
	TRANSPORT_SIZE_REAL          VariableType = 0x08 // 4 bytes (REAL)
	TRANSPORT_SIZE_STRING        VariableType = 0x10 // Variable length (STRING)
	TRANSPORT_SIZE_TIME          VariableType = 0x12 // 4 bytes (TIME)
	TRANSPORT_SIZE_DATE          VariableType = 0x13 // 4 bytes (DATE)
	TRANSPORT_SIZE_DATE_AND_TIME VariableType = 0x14 // 6 bytes (DATE_AND_TIME)
)

func bToTransportSize(b byte) VariableType {
	switch b {
	case 0x01:
		return TRANSPORT_SIZE_BOOL
	case 0x02:
		return TRANSPORT_SIZE_BYTE
	case 0x04:
		return TRANSPORT_SIZE_WORD
	case 0x06:
		return TRANSPORT_SIZE_DWORD
	case 0x08:
		return TRANSPORT_SIZE_REAL
	case 0x10:
		return TRANSPORT_SIZE_STRING
	case 0x12:
		return TRANSPORT_SIZE_TIME
	case 0x13:
		return TRANSPORT_SIZE_DATE
	case 0x14:
		return TRANSPORT_SIZE_DATE_AND_TIME
	default:
		return -1 // Invalid transport size
	}
}

const (
	S7ProtocolID byte = 0x32
)

const (
	S7FuncJob                byte = 0x01
	S7FuncAck                byte = 0x02
	S7FuncAckData            byte = 0x03
	S7FuncUserData           byte = 0x07
	S7FuncSetupCommunication byte = 0xF0

	S7FuncReadVar  byte = 0x04
	S7FuncWriteVar byte = 0x05

	S7FuncStartUpload byte = 0x1D
	S7FuncUpload      byte = 0x1E
	S7FuncEndUpload   byte = 0x1F

	S7FuncStartDownload byte = 0x1A
	S7FuncDownloadBlock byte = 0x1B
	S7FuncDownloadEnd   byte = 0x1C

	S7FuncPlcStop  byte = 0x29
	S7FuncPlcStart byte = 0x28
)

type SyntaxID byte

const (
	S7AnySyntaxID SyntaxID = 0x10
)

// TPKTHeader - TPKT Header Struct
type TPKTHeader struct {
	Version  byte
	Reserved byte
	Length   uint16 // total length (TPKT + COTP + optional data)
}

func (t TPKTHeader) Pack() []byte {
	buff := make([]byte, 0)
	buff = append(buff, t.Version)
	buff = append(buff, t.Reserved)
	buff = append(buff, byte(t.Length>>8), byte(t.Length&0xFF))
	return buff
}

// COTPHeader - COTP Header Struct (minimal 7-byte confirm header)
// 1 byte = D0 = PDU Type: Connect Confirm
// 1 byte = 00 00 = Destination Reference (0x0000)
// 1 byte = 00 01 = Source Reference (0x0001)
// 1 byte = 00 = Class/Options
// 1 byte = C0 01 0A = TPDU size parameter (1 byte length, 0x0A = TPDU size = 1024 bytes)
// 2 byte = C1 02 01 00 = Called TSAP (TSAP = 0x0100, which might refer to the PLC endpoint)
// 2 byte = C2 02 01 02 = Calling TSAP (TSAP = 0x0102, which refers to the client's endpoint)
type COTPHeader struct {
	Length  byte // length of COTP header (excluding TPKT)
	PDUType byte // 0xD0 = Connect Confirm - Connection Type

	EoT byte // End of Transmission (0x00 = no more data) Used After first connection handshake

	DestinationRef uint16
	SourceRef      uint16
	ClassOptions   byte

	TPDUCode   byte // 0xC0 = TPDU size parameter
	TPDULength byte // 0x01 = Length of TPDU size parameter
	TPDUSize   byte // 0x0A = 1024 bytes

	SrcTSAPIdentifier byte   // 0xC1 = Called TSAP Identifier
	SrcTSAPLength     byte   // Length of TSAP (2 bytes)
	CalledTSAP        uint16 // 0x0100 = Called TSAP

	DstTSAPIdentifier byte   // 0xC1 = Called TSAP Identifier
	DstTSAPLength     byte   // Length of TSAP (2 bytes)
	CallingTSAP       uint16 // 0x0102 = Calling TSAP
	// Additional parameters can be added here if needed
	// For example, you might want to include the TPDU size or other options
	// depending on your specific use case. For S7 normally will be empty here for the confirm connection
	// for the response connection, like read/write this will be filled with the S7Header
	Params []byte // optional parameters (like TSAP)
}

func (h *COTPHeader) Encode(pduType byte) ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write single bytes directly
	buf.WriteByte(h.Length)
	buf.WriteByte(pduType)

	if pduType == COTPData { // If is data we need only the EoT
		buf.WriteByte(h.EoT)
		return buf.Bytes(), nil
	}

	// Write uint16 fields in big endian
	if err := binary.Write(buf, binary.BigEndian, h.DestinationRef); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, h.SourceRef); err != nil {
		return nil, err
	}

	buf.WriteByte(h.ClassOptions)

	// TPU size parameter
	buf.WriteByte(h.TPDUCode)
	buf.WriteByte(h.TPDULength)
	buf.WriteByte(h.TPDUSize)

	// Src TSAP parameter
	buf.WriteByte(h.SrcTSAPIdentifier)
	buf.WriteByte(h.SrcTSAPLength)
	if err := binary.Write(buf, binary.BigEndian, h.CalledTSAP); err != nil {
		return nil, err
	}

	// Dst TSAP parameter
	buf.WriteByte(h.DstTSAPIdentifier)
	buf.WriteByte(h.DstTSAPLength)
	if err := binary.Write(buf, binary.BigEndian, h.CallingTSAP); err != nil {
		return nil, err
	}

	// Optional params
	if h.Params != nil {
		buf.Write(h.Params)
	}

	return buf.Bytes(), nil
}

type S7Header struct {
	ProtocolID       byte   // Always 0x32
	ROSCTR           byte   // 0x03 = Ack Data
	RedundancyID     uint16 // 0x0000 Redundancy Identification
	ProtocolDataUnit uint16 // 0x00 = PDU 1280

	ParamLength uint16
	DataLength  uint16
	ErrorClass  byte // 0x00 if success
	ErrorCode   byte // 0x00 if success
}

func (s S7Header) Pack() []byte {
	ret := make([]byte, 0)

	ret = append(ret, s.ProtocolID)
	ret = append(ret, s.ROSCTR)

	ret = append(ret, byte(s.RedundancyID>>8), byte(s.RedundancyID&0xFF))
	ret = append(ret, byte(s.ProtocolDataUnit>>8), byte(s.ProtocolDataUnit&0xFF))
	ret = append(ret, byte(s.ParamLength>>8), byte(s.ParamLength&0xFF))
	ret = append(ret, byte(s.DataLength>>8), byte(s.DataLength&0xFF))

	ret = append(ret, s.ErrorClass)
	ret = append(ret, s.ErrorCode)

	return ret
}

type FuncParamType interface {
	Pack() []byte
}

type S7Request struct {
	FunctionCode byte // 0x04 = Read Var

	DataSection []byte

	FuncParam FuncParamType // Store the function parameters given FunctionCode definition, see unpackS7Function routine
}

func (s S7Request) Pack() []byte {
	ret := make([]byte, 0)
	ret = append(ret, s.FunctionCode)
	ret = append(ret, s.FuncParam.Pack()...)
	return ret
}

type S7Response struct {
	FunctionCode byte // 0x04 = Read Var

	DataSection []byte

	FuncParam FuncParamType // Store the function parameters given FunctionCode definition, see unpackS7Function routine
}

func (s S7Response) Pack() []byte {
	ret := make([]byte, 0)
	ret = append(ret, s.FunctionCode)
	ret = append(ret, s.FuncParam.Pack()...)
	return ret
}

type S7ParamReadDBErrorResponse struct {
	ItemCount byte                    // 0x01 = Number of items in the response
	Items     []S7VarRequestItemError // Array of results
}

func (s S7ParamReadDBErrorResponse) Pack() []byte {
	ret := make([]byte, 0)
	ret = append(ret, s.ItemCount)
	for _, item := range s.Items {
		ret = append(ret, item.ReturnCode)
		ret = append(ret, item.TransportSize)
		ret = append(ret, byte(item.Length>>8), byte(item.Length&0xFF))
	}
	return ret
}

type S7ParamSetupCommunication struct {
	Reserved     byte   // 0x00 = Reserved
	MaxAmqCaller uint16 // Max number of simultaneous caller connections
	MaxAmqCallee uint16 // Max number of simultaneous callee connections
	PduLength    uint16 // Maximum PDU Length (bytes)
}

func (s S7ParamSetupCommunication) Pack() []byte {
	buffer := new(bytes.Buffer)
	if err := binary.Write(buffer, binary.BigEndian, s.Reserved); err != nil {
		log.Println("Error writing MaxSimultaneous:", err)
		return nil
	}
	if err := binary.Write(buffer, binary.BigEndian, s.MaxAmqCaller); err != nil {
		log.Println("Error writing ReservedFlag:", err)
		return nil
	}

	if err := binary.Write(buffer, binary.BigEndian, s.MaxAmqCallee); err != nil {
		log.Println("Error writing MaxAmqCaller:", err)
		return nil
	}
	if err := binary.Write(buffer, binary.BigEndian, s.PduLength); err != nil {
		log.Println("Error writing PduLength:", err)
		return nil
	}
	return buffer.Bytes()
}

type S7VarRequestItem struct {
	VarSpec       byte     // 0x01 = Variable Specification
	LenAddrSpec   byte     // 0x0A = Address Specification Length, defines the remaining bytes size
	SyntaxID      SyntaxID // 0x10 = S7Any
	TransportSize byte     // 0x02 = BYTE, WORD, etc
	Length        uint16   // Length of the variable (e.g., 2 bytes for INT, 4 bytes for REAL, 1 byte for BYTE, etc)
	DBNumber      uint16   // DB number (e.g., 200)
	Area          byte     // Area (0x84 = DB, 0x83 = Inputs, 0x81 = Outputs, etc)
	Address       uint32   // Address inside area (but only 24 bits used!)

	ByteOffset uint32
	BitOffset  byte
}
type S7ParamReadVar struct {
	Items []S7VarRequestItem
}

func (s S7ParamReadVar) Pack() []byte {
	ret := make([]byte, 0)
	for _, item := range s.Items {
		ret = append(ret, byte(item.SyntaxID))
		ret = append(ret, item.TransportSize)
		ret = append(ret, byte(item.Length>>8), byte(item.Length&0xFF))
		ret = append(ret, byte(item.DBNumber>>8), byte(item.DBNumber&0xFF))
		ret = append(ret, item.Area)
		ret = append(ret, byte(item.Address>>16), byte(item.Address>>8), byte(item.Address&0xFF))
	}
	return ret
}

type S7IItemReturnCode byte

const (
	// Success
	S7ItemReturnCodeSuccess S7IItemReturnCode = 0xFF

	// Errors
	S7ItemReturnCodeHardwareFault        S7IItemReturnCode = 0x01
	S7ItemReturnCodeAccessingNotAllowed  S7IItemReturnCode = 0x03
	S7ItemReturnCodeInvalidAddress       S7IItemReturnCode = 0x05
	S7ItemReturnCodeDataTypeInconsistent S7IItemReturnCode = 0x06
	S7ItemReturnCodeObjectNotAvailable   S7IItemReturnCode = 0x0A
)

type S7VarResponseItem struct {
	ReturnCode    S7IItemReturnCode
	TransportSize byte
	Length        uint16
	Data          []byte
}

func (s S7VarResponseItem) Pack() []byte {
	ret := make([]byte, 0)
	ret = append(ret, byte(s.ReturnCode))
	ret = append(ret, s.TransportSize)
	ret = append(ret, byte(s.Length>>8), byte(s.Length&0xFF))
	ret = append(ret, s.Data...)
	return ret
}

type S7ResponseReadVars struct {
	ItemCount byte // 0x01 = Number of items in the response
	Items     []*S7VarResponseItem
}

func (s S7ResponseReadVars) Pack() []byte {
	ret := make([]byte, 0)
	ret = append(ret, s.ItemCount)
	for _, item := range s.Items {
		ret = append(ret, byte(item.ReturnCode))
		ret = append(ret, item.TransportSize)
		ret = append(ret, byte(item.Length>>8), byte(item.Length&0xFF))
		ret = append(ret, item.Data...)
	}
	return ret
}

type S7VarRequestItemError struct {
	ReturnCode    byte   // 0x00 = OK, 0xFF = Error
	TransportSize byte   // 0x02=WORD, 0x03=BYTE, 0x04=REAL, etc.
	Length        uint16 // Data payload (empty if ReturnCode != 0x00)
}

type S7ParamReadVarResponseError struct {
	Items []S7VarRequestItemError
}

func (s S7ParamReadVarResponseError) Pack() []byte {
	ret := make([]byte, 0)
	for _, item := range s.Items {
		ret = append(ret, item.ReturnCode)
		ret = append(ret, item.TransportSize)
		ret = append(ret, byte(item.Length>>8), byte(item.Length&0xFF))
	}
	return ret
}

type Message struct {
	TPKTHeader TPKTHeader
	COTPHeader COTPHeader
	S7Header   *S7Header

	S7Request  *S7Request
	S7Response *S7Response
}

func (m *Message) Pack(pdyType byte) ([]byte, error) {
	buff := make([]byte, 0)

	// Create the TPK Header
	buff = m.TPKTHeader.Pack()

	cotpHeader, err := m.COTPHeader.Encode(pdyType)
	if err != nil {
		return nil, err
	}
	buff = append(buff, cotpHeader...)

	if m.S7Header != nil {
		buff = append(buff, m.S7Header.Pack()...)
	}
	if m.S7Request != nil {
		buff = append(buff, m.S7Request.Pack()...)
	}
	if m.S7Response != nil {
		buff = append(buff, m.S7Response.Pack()...)
	}

	// Calculate and set the TPKT header length
	binary.BigEndian.PutUint16(buff[2:4], uint16(len(buff)))

	return buff, nil
}

// unpack - Unpack the byte array into a Message struct
// TPKT Header (4 bytes)
// | Version | Reserved | Length (Big Endian) |
// |   1     |    1     |        2           |
func unpack(b []byte) (*Message, error) {

	ret := Message{}

	if len(b) < 4 { // Validate the TPKT header first
		return nil, errors.New("TPKT header is too short")
	}
	ret.TPKTHeader.Version = b[0]
	ret.TPKTHeader.Reserved = b[1]
	ret.TPKTHeader.Length = uint16(b[2])<<8 | uint16(b[3])
	if len(b) < int(ret.TPKTHeader.Length) {
		return nil, errors.New("TPKT header length is too short")
	}

	// COTP Header
	if len(b) < 22 { // Validate the COTP header
		return nil, errors.New("COTP header is too short")
	}
	ret.COTPHeader.Length = b[4]
	ret.COTPHeader.PDUType = b[5]

	if ret.COTPHeader.PDUType == COTPConnectionRequest {
		// COTP Header (17 bytes)
		// | Length | PDU Type | Destination Ref | Source Ref | Class/Options | Parameters |
		// |   1    |    1     |       2         |     2     |      1        |    Varies  |
		ret.COTPHeader.DestinationRef = uint16(b[6])<<8 | uint16(b[7])
		ret.COTPHeader.SourceRef = uint16(b[8])<<8 | uint16(b[9])
		ret.COTPHeader.ClassOptions = b[10]

		ret.COTPHeader.TPDUCode = b[11]
		ret.COTPHeader.TPDULength = b[12]
		ret.COTPHeader.TPDUSize = b[13]

		ret.COTPHeader.SrcTSAPIdentifier = b[14]
		ret.COTPHeader.SrcTSAPLength = b[15]
		ret.COTPHeader.CalledTSAP = uint16(b[16])<<8 | uint16(b[17])

		ret.COTPHeader.DstTSAPIdentifier = b[18]
		ret.COTPHeader.DstTSAPLength = b[19]
		ret.COTPHeader.CallingTSAP = uint16(b[20])<<8 | uint16(b[21])

		if len(b) > 22 {
			ret.COTPHeader.Params = b[22:ret.TPKTHeader.Length]
		}
	} else {
		switch ret.COTPHeader.PDUType {
		case COTPData: // We need to collect the EoT instead all the data
			ret.COTPHeader.EoT = b[6] // EoT is the 7th byte of the COTP header
			if b[7] == S7ProtocolID { // Magic number for S7
				s7B := b[7:]
				s7 := &S7Header{
					ProtocolID:       s7B[0],
					ROSCTR:           s7B[1],
					RedundancyID:     binary.BigEndian.Uint16(s7B[2:4]),
					ProtocolDataUnit: binary.BigEndian.Uint16(s7B[4:6]),
					ParamLength:      binary.BigEndian.Uint16(s7B[6:8]),
					DataLength:       binary.BigEndian.Uint16(s7B[8:10]),
				}

				s7Request := &S7Request{
					FunctionCode: s7B[10],  // Return the function code to be used later
					DataSection:  s7B[11:], // Store the data section, just in case
				}
				ret.S7Header = s7
				ret.S7Request = s7Request

				funcParam, err := unpackS7Function(s7Request)
				if err != nil {
					return nil, err
				}
				ret.S7Request.FuncParam = funcParam

			} else {
				// Return a error if the PDU type is not S7
				return nil, errors.New("the S7 header is not valid")
			}
		default:
			// After the initial confirmation request the COTP PDUType will be always 0xE0 (COTPData), if not
			// the client is sending wrong data request
			return nil, errors.New("COTP header is not valid")
		}
	}

	return &ret, nil
}

func unpackS7Function(s7Request *S7Request) (FuncParamType, error) {
	b := s7Request.DataSection
	switch s7Request.FunctionCode {
	case S7FuncSetupCommunication: // Setup communication function
		switch len(b) {
		case 7:
			return &S7ParamSetupCommunication{
				Reserved:     b[0],
				MaxAmqCaller: binary.BigEndian.Uint16(b[1:3]),
				MaxAmqCallee: binary.BigEndian.Uint16(b[3:5]),
				PduLength:    binary.BigEndian.Uint16(b[5:7]),
			}, nil
		default:
			return nil, errors.New("S7FuncSetupCommunication: data length is not valid")
		}
	case S7FuncReadVar:

		// | Item Count --> The "b" buffer contains data from here
		// |  1 byte
		// |  [0]
		// The items now repeat by the amount of times defined in the item count
		// | VarSpec | Len  | --> Len specifies the length of the address spec
		// | 1 byte  |1 byte|
		// |   [1]   | [2]  |
		// So the remaining bytes need to match the length of the address spec
		// 					| SyntaxID | TranspSize | Length | DB Number | Area   |  Address |
		// 					|  1 byte  |   1 byte   |  2 B   | 2 bytes   | 1 byte |  3 bytes |
		// 					|   [0]    |     [1]    | [2-3]  |  [4-5]    |  [6]   |   [7-9]  |
		// Each item has a total of: 12 bytes
		// all indexes are relative to the start of the item to the end of the item and the subsequent items
		// need an offset given the item count for each iteration
		itemsCount := int(b[0]) // First byte = FunctionCode (should be 0x04), Second byte = number of items
		pos := 1                // Start reading after item count

		var items []S7VarRequestItem

		for i := 0; i < itemsCount; i++ {
			// if the size of the buffer is less than the position + 2 bytes we do not have any item to parse
			if len(b[pos:]) <= 2 {
				return nil, fmt.Errorf("not enough b to parse item %d", i)
			}

			item := S7VarRequestItem{
				VarSpec:     b[pos],
				LenAddrSpec: b[pos+1], // Normally 0x0A
			}

			if len(b[pos+1:]) < int(item.LenAddrSpec) {
				return nil, fmt.Errorf("not enough b to parse item %d", i)
			}

			addressSpec := b[(pos + 2) : (pos+2)+int(item.LenAddrSpec)]

			item.SyntaxID = SyntaxID(addressSpec[0])
			item.TransportSize = addressSpec[1]
			item.Length = binary.BigEndian.Uint16(addressSpec[2:4])
			item.DBNumber = binary.BigEndian.Uint16(addressSpec[4:6])
			item.Area = addressSpec[6]

			// The S7 uses a 3 byte address, so we need to shift the bytes to the left
			// this will be the offset address on the database bytes arrays
			item.Address = uint32(addressSpec[7])<<16 | uint32(addressSpec[8])<<8 | uint32(addressSpec[9])

			item.ByteOffset = item.Address / 8
			item.BitOffset = byte(item.Address % 8)

			items = append(items, item)

			// Offset the items by the last item address specification + 2 bytes
			pos += 2 + int(item.LenAddrSpec)
		}

		return &S7ParamReadVar{
			Items: items,
		}, nil
	}
	return nil, errors.New("function code not supported")
}

func getS7ParamSetupCommunicationResponse(msg *Message) (*Message, error) {
	// verify if the FuncParam is S7ParamSetupCommunication
	if msg.S7Request.FuncParam == nil {
		return nil, errors.New("FuncParam is nil")
	}
	if reflect.TypeOf(msg.S7Request.FuncParam) != reflect.TypeOf(&S7ParamSetupCommunication{}) {
		return nil, errors.New("FuncParam is not S7ParamSetupCommunication")
	}
	ret := &Message{
		TPKTHeader: msg.TPKTHeader,
		COTPHeader: msg.COTPHeader,
		S7Header: &S7Header{
			ProtocolID:   S7ProtocolID,
			ROSCTR:       S7FuncAckData,
			RedundancyID: 0,
			ParamLength:  4,
			DataLength:   8,
			ErrorClass:   0,
			ErrorCode:    0,
		},
		S7Request: &S7Request{
			FunctionCode: 0,
			DataSection:  nil,
			FuncParam: &S7ParamSetupCommunication{
				MaxAmqCaller: 0,
				PduLength:    msg.S7Request.FuncParam.(*S7ParamSetupCommunication).PduLength,
			},
		},
	}
	return ret, nil
}

func printBin(data []byte) string {
	s := fmt.Sprintf("Binary: [")
	for _, b := range data {
		s += fmt.Sprintf("0x%02X ", b)
	}
	s += fmt.Sprintf("]\n")
	return s
}
