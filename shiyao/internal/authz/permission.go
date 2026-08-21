package authz

type Permission string

const (
	PermissionSandboxRead  Permission = "sandbox:read"
	PermissionSandboxWrite Permission = "sandbox:write"
	PermissionUserRead     Permission = "user:read"
	PermissionUserWrite    Permission = "user:write"
	PermissionTokenRead    Permission = "token:read"
	PermissionTokenWrite   Permission = "token:write"
)

func (p Permission) Valid() bool {
	switch p {
	case PermissionSandboxRead,
		PermissionSandboxWrite,
		PermissionUserRead,
		PermissionUserWrite,
		PermissionTokenRead,
		PermissionTokenWrite:
		return true
	default:
		return false
	}
}
