package authz

const ResourceReconcile = "reconcile"

var (
	ReconcileRead      = Permission{Resource: ResourceReconcile, Action: ActionRead}
	ReconcileOperate   = Permission{Resource: ResourceReconcile, Action: ActionOperate}
	ReconcileConfigure = Permission{Resource: ResourceReconcile, Action: ActionSensitiveWrite}
	ReconcileExport    = Permission{Resource: ResourceReconcile, Action: "export"}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceReconcile,
		LabelKey: "Reconciliation",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read reconciliation",
				DescriptionKey: "View reconciliation runs, request differences, and account summaries.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionOperate,
				LabelKey:       "Operate reconciliation",
				DescriptionKey: "Run diagnostics, start reconciliation, and retry failed runs.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionSensitiveWrite,
				LabelKey:       "Configure reconciliation",
				DescriptionKey: "Configure IAM roles, AWS data sources, schedules, and channel mappings.",
			},
			{
				Action:         "export",
				LabelKey:       "Export reconciliation",
				DescriptionKey: "Export reconciliation request and cost data.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
		},
	})
}
