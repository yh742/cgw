package cgw

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

// EntityIdentifier is struct for data that contains entitypair
type EntityIdentifier interface {
	GetEntityPair() *EntityPair
}

// DisconnectRequest is the json used for request
type DisconnectRequest struct {
	EntityPair
	ReasonCode ReasonCode `json:"reasonCode"`
	NextServer string     `json:"nextServer"`
}

// IsValid check is any of the fields are empty or not valid
func (tokReq *DisconnectRequest) IsValid() bool {
	if !tokReq.EntityPair.IsValid() {
		return false
	}
	// check if the reason code exists
	switch tokReq.ReasonCode {
	case Reauthenticate:
	case NotAuthorized:
	case Expiration:
	case Handover:
	case Idle:
	default:
		return false
	}
	return true
}

// GetEntityPair gets the entity pair in the struct
func (tokReq *DisconnectRequest) GetEntityPair() *EntityPair {
	return &tokReq.EntityPair
}

// ValidityChecker for structs that checks fields
type ValidityChecker interface {
	IsValid() bool
}

// ValidateTokenRequest is the json used for caas validation request
type ValidateTokenRequest struct {
	EntityTokenRequest
	MEC string `json:"mec"`
}

// EntityTokenRequest is the json used for deleting entity requests
type EntityTokenRequest struct {
	EntityPair
	Token string `json:"token"`
}

// IsValid check is any of the fields are empty
func (tokReq *EntityTokenRequest) IsValid() bool {
	if !tokReq.EntityPair.IsValid() || IsEmpty(tokReq.Token) {
		return false
	}
	return true
}

// GetEntityPair gets the entity pair in the struct
func (tokReq *EntityTokenRequest) GetEntityPair() *EntityPair {
	return &tokReq.EntityPair
}

// EntityPair is the entity/entityid combo
type EntityPair struct {
	Entity   string `json:"entity"`
	EntityID string `json:"entityid"`
}

// IsValid check is any of the fields are empty
func (ep *EntityPair) IsValid() bool {
	if IsEmpty(ep.Entity) || IsEmpty(ep.EntityID) {
		return false
	}
	return true
}

// CreateKey builds a key from entity pair
func (ep *EntityPair) CreateKey() string {
	return HyphenConcat(ep.Entity, ep.EntityID)
}
