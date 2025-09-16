package goprotos7

type S7ErrorClass byte

const (
	ErrorClassNoError                 S7ErrorClass = 0x00 // Success / No Error
	ErrorClassApplicationRelationship S7ErrorClass = 0x81 // Application relation error (connection, partner, syntax)
	ErrorClassObjectDefinition        S7ErrorClass = 0x82 // Object definition error (DB not found, wrong address)
	ErrorClassNoResourcesAvailable    S7ErrorClass = 0x83 // Out of memory, slots exhausted, busy
	ErrorClassServiceError            S7ErrorClass = 0x84 // Service error (not implemented, unavailable)
	ErrorClassAccessError             S7ErrorClass = 0x85 // Access error (permission denied, protected area)
	ErrorClassApplicationRelation     S7ErrorClass = 0x05 // Application relation error (newer S7COMM, equivalent to 0x81)
	ErrorClassReserved                S7ErrorClass = 0x87 // Reserved (sometimes seen on proprietary S7 extension)
	ErrorClassManufacturerSpecific    S7ErrorClass = 0x0A // Manufacturer-specific errors
	ErrorClassGeneralError            S7ErrorClass = 0xFF // Unknown or catch-all error
)

type S7ErrorCode byte

const (
	ErrorCodeNoError                S7ErrorCode = 0x00 // No error
	ErrorCodeFunctionNotExists      S7ErrorCode = 0x01 // Function not supported or does not exist
	ErrorCodeAccessingObject        S7ErrorCode = 0x03 // Object not available
	ErrorCodeAddressOutOfRange      S7ErrorCode = 0x05 // Address invalid or out of range
	ErrorCodeDataTypeNotSupported   S7ErrorCode = 0x06 // Type or transport size not supported
	ErrorCodeDataTypeInconsistent   S7ErrorCode = 0x07 // Data inconsistent (e.g., length mismatch)
	ErrorCodeObjectAlreadyExists    S7ErrorCode = 0x0A // Trying to create an existing object
	ErrorCodeObjectDoesNotExist     S7ErrorCode = 0x0B // Deleting or accessing nonexistent object
	ErrorCodeNoPrivilege            S7ErrorCode = 0x0C // Access denied
	ErrorCodeInsufficientResources  S7ErrorCode = 0x0D // No resources (e.g., too many connections)
	ErrorCodeInvalidParameter       S7ErrorCode = 0x0E // Parameter invalid
	ErrorCodeServiceNotAvailable    S7ErrorCode = 0x0F // Service temporarily unavailable
	ErrorCodeAccessDenied           S7ErrorCode = 0x10 // Client is not allowed to execute this
	ErrorCodeFunctionNotImplemented S7ErrorCode = 0x20 // Function not implemented in the CPU
	ErrorCodeInvalidBlockType       S7ErrorCode = 0x21 // Block type invalid
	ErrorCodeInvalidBlockNumber     S7ErrorCode = 0x22 // Block number invalid
	ErrorCodeInvalidBlockVersion    S7ErrorCode = 0x23 // Block version mismatch
	ErrorCodeBlockNotLoadable       S7ErrorCode = 0x24 // Block cannot be loaded to the CPU
	ErrorCodeNotImplemented         S7ErrorCode = 0xFF // General catch-all not implemented
)
