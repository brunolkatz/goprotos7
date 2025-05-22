package goprotos7

type S7ErrorClass byte

const (
	ErrorClassNoError                 S7ErrorClass = 0x00 // Success
	ErrorClassApplicationRelationship S7ErrorClass = 0x81 // Application error (job, syntax, area)
	ErrorClassObjectDefinition        S7ErrorClass = 0x82 // DB not found, wrong object
	ErrorClassNoResourcesAvailable    S7ErrorClass = 0x83 // Out of memory, busy
	ErrorClassServiceError            S7ErrorClass = 0x84 // Invalid service or action
	ErrorClassAccessError             S7ErrorClass = 0x85 // Permission denied
)

type S7ErrorCode byte

const (
	ErrorCodeNoError              S7ErrorCode = 0x00 // No error
	ErrorCodeHardwareFault        S7ErrorCode = 0x01 // CPU failure, hardware fault
	ErrorCodeAccessingObject      S7ErrorCode = 0x03 // Object not available (e.g., DB missing)
	ErrorCodeAddressOutOfRange    S7ErrorCode = 0x05 // Address out of range
	ErrorCodeDataTypeNotSupported S7ErrorCode = 0x06 // TransportSize or type not supported
	ErrorCodeDataTypeInconsistent S7ErrorCode = 0x07 // Data inconsistent (e.g., length mismatch)
	ErrorCodeObjectAlreadyExists  S7ErrorCode = 0x0A // Trying to create an existing object
	ErrorCodeObjectDoesNotExist   S7ErrorCode = 0x0B // Delete non-existent object
	ErrorCodeNoPrivilege          S7ErrorCode = 0x0C // Access denied
)

type S7ReturnCode byte

const (
	S7ReturnCodeSuccess              S7ReturnCode = 0x00
	S7ReturnCodeAddressInvalid       S7ReturnCode = 0x05
	S7ReturnCodeDataTypeInconsistent S7ReturnCode = 0x0A
	S7ReturnCodeAccessingNotPossible S7ReturnCode = 0xFF
)

func GetErrorReturnMessage(classError S7ErrorClass, errCode S7ErrorCode, msg *Message) *Message {
	ret := &Message{
		TPKTHeader: msg.TPKTHeader, // 4 bytes
		COTPHeader: msg.COTPHeader, // 3 bytes
		S7Header: &S7Header{
			ProtocolID:   S7ProtocolID,     // 1 byte
			ROSCTR:       S7FuncAckData,    // 1 byte
			RedundancyId: 0,                // 2 byte
			ParamLength:  4,                // 2 byte
			DataLength:   0,                // 2 byte
			ErrorClass:   byte(classError), // 1 byte
			ErrorCode:    byte(errCode),    // 1 byte
		}, // 18 bytes total
		S7Response: &S7Response{
			FunctionCode: msg.S7Request.FunctionCode,
		},
	}

	switch msg.S7Request.FunctionCode {
	case S7FuncReadVar:
		if r, ok := msg.S7Request.FuncParam.(*S7ParamReadVar); ok {
			funcParam := &S7ParamReadDBErrorResponse{
				ItemCount: byte(len(r.Items)),
				Items:     make([]S7VarRequestItemError, 0),
			}
			for _, _ = range r.Items {
				funcParam.Items = append(funcParam.Items, S7VarRequestItemError{
					ReturnCode:    byte(S7ReturnCodeAddressInvalid),
					TransportSize: 0,
					Length:        0,
				})
			}
			ret.S7Response.FuncParam = funcParam
		}
	}
	return ret
}

type S7ReadVarHeader struct {
	ItemsCount byte // Number of items in the request
	Items      []*S7VarRequestItem
}
