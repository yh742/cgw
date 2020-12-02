package main

// ReasonCode explains the disconnection reason
type ReasonCode byte

const (
	Reauthenticate ReasonCode = 0x19
	NotAuthorized  ReasonCode = 0x87
	Expiration     ReasonCode = 0x0A
	RateTooHigh    ReasonCode = 0x96
	Handover       ReasonCode = 0x9C
	Idle           ReasonCode = 0x98
)
