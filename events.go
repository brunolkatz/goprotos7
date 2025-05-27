package goprotos7

import (
	"errors"
	"log"
	"net"
)

func (s *Server) eventS7FuncReadVar(msg *Message, conn net.Conn) {
	if msg == nil {
		log.Printf("[SERVER] Received nil message in ack data event")
		return
	}

	funcParam := msg.S7Request.FuncParam.(*S7ParamReadVar)

	res := &Message{
		TPKTHeader: msg.TPKTHeader,
		COTPHeader: msg.COTPHeader,
		S7Header: &S7Header{
			ProtocolID:       S7ProtocolID,
			ROSCTR:           S7FuncAckData,
			RedundancyID:     msg.S7Header.RedundancyID,
			ProtocolDataUnit: msg.S7Header.ProtocolDataUnit,
			ParamLength:      msg.S7Header.ParamLength,
			DataLength:       msg.S7Header.DataLength,
			ErrorClass:       byte(ErrorClassNoError),
			ErrorCode:        byte(ErrorCodeNoError),
		},
		S7Response: &S7Response{
			FunctionCode: S7FuncReadVar,
			FuncParam: &S7ResponseReadVars{
				ItemCount: byte(len(funcParam.Items)),
				Items:     make([]*S7VarResponseItem, len(funcParam.Items)-1),
			},
		},
	}

	for _, i := range funcParam.Items {
		v, err := s.getDBValue(i.DBNumber, i.TransportSize, i.ByteOffset, i.BitOffset, i.Length)
		if err != nil {
			switch {
			case errors.Is(err, ErrorOutOfBounds):
				res.S7Response.FuncParam.(*S7ResponseReadVars).Items = append(res.S7Response.FuncParam.(*S7ResponseReadVars).Items, &S7VarResponseItem{
					ReturnCode:    S7ItemReturnCodeInvalidAddress,
					TransportSize: 0x00,
					Length:        0,
					Data:          make([]byte, 0),
				})
			default:
				res.S7Response.FuncParam.(*S7ResponseReadVars).Items = append(res.S7Response.FuncParam.(*S7ResponseReadVars).Items, &S7VarResponseItem{
					ReturnCode:    S7ItemReturnCodeInvalidAddress,
					TransportSize: 0x00,
					Length:        0,
					Data:          make([]byte, 0),
				})
			}
		}
		res.S7Response.FuncParam.(*S7ResponseReadVars).Items = append(res.S7Response.FuncParam.(*S7ResponseReadVars).Items, &S7VarResponseItem{
			ReturnCode:    S7ItemReturnCodeSuccess,
			TransportSize: i.TransportSize,
			Length:        uint16(len(v)),
			Data:          v,
		})
	}

	// Calculate the data length
	dt := 0
	for _, i := range res.S7Response.FuncParam.(*S7ResponseReadVars).Items {
		dt += len(i.Pack())
	}
	res.S7Header.DataLength = uint16(dt)

	// Write to the client
	ack, err := res.Pack(COTPData)
	if err != nil {
		log.Printf("[SERVER] Error packing response: %s", err)
		return
	}
	_, err = conn.Write(ack)
	if err != nil {
		log.Printf("[SERVER] Error writing response: %s", err)
		return
	}
	return
}
