package vmm

type State uint8

const (
	StateCreated State = iota
	StateConfiguring
	StateConfigured
	StateRunning
	StateStopping
	StateStopped
	StateCleanupFailed
)

func (s State) String() string {
	switch s {
	case StateCreated:
		return "created"
	case StateConfiguring:
		return "configuring"
	case StateConfigured:
		return "configured"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateCleanupFailed:
		return "cleanup-failed"
	default:
		return "unknown"
	}
}
