package outbox

import "context"

// Publisher publishes an outbox message to the message bus.
type Publisher interface {
	Publish(context.Context, string, []byte, map[string]string, string) error
}

type publishErrorClass int

const (
	publishErrorUnknown publishErrorClass = iota
	publishErrorRetryable
	publishErrorPermanent
)

type permanentPublishError interface {
	Permanent() bool
}

type retryablePublishError interface {
	Retryable() bool
}

type codedPublishError interface {
	FailureCode() string
}

func classifyPublishError(err error) (publishErrorClass, string) {
	if err == nil {
		return publishErrorUnknown, ""
	}
	code := "unknown"
	if coded, ok := err.(codedPublishError); ok && coded.FailureCode() != "" {
		code = coded.FailureCode()
	}
	if permanent, ok := err.(permanentPublishError); ok && permanent.Permanent() {
		return publishErrorPermanent, code
	}
	if retryable, ok := err.(retryablePublishError); ok && retryable.Retryable() {
		return publishErrorRetryable, code
	}
	return publishErrorUnknown, code
}
