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
