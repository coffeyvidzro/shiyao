package sandbox

import "context"

type VMManager interface {
	ProvisionVM(context.Context, string) error
	DestroyVM(context.Context, string) error
}
