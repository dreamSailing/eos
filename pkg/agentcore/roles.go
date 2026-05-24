package agentcore

import rolecfg "github.com/dreamSailing/eos/pkg/agentcore/roles"

type ContextStrategy = rolecfg.ContextStrategy

const (
	ContextShared      = rolecfg.ContextShared
	ContextIndependent = rolecfg.ContextIndependent
	ContextHybrid      = rolecfg.ContextHybrid
)

type Role = rolecfg.RoleConfig
type RoleConfig = rolecfg.Document
type RoleConfigPaths = rolecfg.ConfigPaths
type RoleRegistry = rolecfg.Registry

func NewRoleRegistry(defaults []Role) (*RoleRegistry, error) {
	return rolecfg.NewRegistry(defaults)
}

func NewDefaultRoleRegistry() *RoleRegistry {
	return rolecfg.NewDefaultRegistry()
}

func LoadRoleRegistry(paths ...string) (*RoleRegistry, error) {
	return rolecfg.LoadRegistry(paths...)
}

func LoadRoleRegistryWithPaths(paths RoleConfigPaths) (*RoleRegistry, error) {
	return rolecfg.LoadRegistryWithPaths(paths)
}

func DefaultRoleConfigPaths(workspaceRoot string) RoleConfigPaths {
	return rolecfg.DefaultConfigPaths(workspaceRoot)
}

func BuiltinRoles() []Role {
	return rolecfg.BuiltinRoles()
}

func NormalizeRoleID(id string) string {
	return rolecfg.NormalizeRoleID(id)
}
