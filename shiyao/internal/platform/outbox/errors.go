package outbox

import "errors"

var (
	ErrClaimLost      = errors.New("outbox event claim was lost")
	ErrNotQuarantined = errors.New("outbox event is not quarantined")
)
