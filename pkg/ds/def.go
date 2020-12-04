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

// EntityTokenPair is the json used for request
type EntityTokenPair struct {
	Token    string `json:"token"`
	EntityID string `json:"entityid"`
}

// DeleteEntityRequest is the json used for deleting entity requests
type DeleteEntityRequest struct {
	EntityTokenPair
	Entity string `json:"entity"`
}
