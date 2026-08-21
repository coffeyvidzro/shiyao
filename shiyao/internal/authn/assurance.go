package authn

type AssuranceLevel uint8

const (
	AssuranceUnknown AssuranceLevel = iota
	AssurancePassword
	AssuranceOTP
	AssuranceMFA
)
