package vsock

// Protocol and transport limits are centralized here so host and guest code
// can review the VSOCK resource boundary independently from VM lifecycle code.
const (
	MaxFrameBytes = MaxRequestBytes
	MaxResponseBytes = MaxOutputBytes
)
