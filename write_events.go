package goprotos7

import (
	"log"
	"net"
)

func (c *Connection) eventS7FuncWriteVar(msg *Message, conn net.Conn) {
	if msg == nil || msg.S7Request == nil || msg.S7Request.FuncParam == nil {
		return
	}

	funcParam, ok := msg.S7Request.FuncParam.(*S7ParamWriteVar)
	if !ok {
		ret := getS7FunctionCodeNotSupportedResponse(msg)
		ack, err := ret.Pack(COTPData)
		if err == nil {
			_, _ = conn.Write(ack)
		}
		return
	}

	results := make([]S7IItemReturnCode, 0, len(funcParam.Items))
	for i := range funcParam.Items {
		item := funcParam.Items[i]
		if i >= len(funcParam.DataItems) {
			results = append(results, S7ItemReturnCodeDataTypeInconsistent)
			continue
		}
		dataItem := funcParam.DataItems[i]

		// We only persist DB area writes into DB{n}.bin files.
		if item.Area != 0x84 {
			results = append(results, S7ItemReturnCodeObjectNotAvailable)
			continue
		}

		transport := TransportSizeByte
		if item.TransportSize == 0x03 {
			transport = TransportSizeBit
		}
		err := c.setDBValue(item.DBNumber, transport, item.ByteOffset, item.BitOffset, dataItem.Data)
		if err != nil {
			if err == ErrorOutOfBounds {
				results = append(results, S7ItemReturnCodeInvalidAddress)
				continue
			}
			results = append(results, S7ItemReturnCodeHardwareFault)
			continue
		}
		results = append(results, S7ItemReturnCodeSuccess)
	}

	res := &Message{
		TPKTHeader: msg.TPKTHeader,
		COTPHeader: msg.COTPHeader,
		S7Header: &S7Header{
			ProtocolID:       S7ProtocolID,
			ROSCTR:           S7FuncAckData,
			RedundancyID:     msg.S7Header.RedundancyID,
			ProtocolDataUnit: msg.S7Header.ProtocolDataUnit,
			ParamLength:      2,
			DataLength:       uint16(len(results)),
			ErrorClass:       byte(ErrorClassNoError),
			ErrorCode:        byte(ErrorCodeNoError),
		},
		S7Response: &S7Response{
			FunctionCode: S7FuncWriteVar,
			FuncParam: &S7ResponseWriteVar{
				ItemCount: byte(len(results)),
				Results:   results,
			},
		},
	}

	ack, err := res.Pack(COTPData)
	if err != nil {
		log.Printf("[SERVER] Error packing WriteVar response: %s", err)
		return
	}
	_, err = conn.Write(ack)
	if err != nil {
		log.Printf("[SERVER] Error writing WriteVar response: %s", err)
	}
}
