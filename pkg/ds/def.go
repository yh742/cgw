package ds

// ReasonCode explains the disconnection reason
type ReasonCode byte

// ReasonCode enumerations
const (
	Reauthenticate ReasonCode = 0x19
	NotAuthorized  ReasonCode = 0x87
	Expiration     ReasonCode = 0x0A
	RateTooHigh    ReasonCode = 0x96
	Handover       ReasonCode = 0x9C
	Idle           ReasonCode = 0x98
)

// DisconnectRequest is the json used for request
type DisconnectRequest struct {
	Entity     string     `json:"entity"`
	EntityID   string     `json:"entityid"`
	ReasonCode ReasonCode `json:"reasonCode"`
	NextServer string     `json:"nextServer"`
}

// FieldsChecker for structs that checks fields
type FieldsChecker interface {
	FieldsEmpty() bool
}

// ValidateTokenRequest is the json used for caas validation request
type ValidateTokenRequest struct {
	EntityTokenRequest
	MEC string `json:"mec"`
}

// EntityTokenRequest is the json used for deleting entity requests
type EntityTokenRequest struct {
	EntityIDStruct
	Token  string `json:"token"`
	Entity string `json:"entity"`
}

// FieldsEmpty check is any of the fields are empty
func (tokReq *EntityTokenRequest) FieldsEmpty() bool {
	if IsEmpty(tokReq.Entity) || IsEmpty(tokReq.EntityID) || IsEmpty(tokReq.Token) {
		return true
	}
	return false
}

// EntityIDStruct is the entityID
type EntityIDStruct struct {
	EntityID string `json:"entityid"`
}
