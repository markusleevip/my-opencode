package types

type PermissionsMode string

const (
	ActManual PermissionsMode = "act_manual"
	ActAuto   PermissionsMode = "act_auto"
	Plan      PermissionsMode = "plan"
)
