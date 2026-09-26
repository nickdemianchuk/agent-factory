package v1alpha1

// AgentBoxIDLabel is set on every resource of an agent box.
const AgentBoxIDLabel = "agentfactory.io/agent-box-id"

// BoxFinalizer guards deletion of the box namespace.
const BoxFinalizer = "agentbox.agentfactory.io/finalizer"

// ConditionReady is the condition type for readiness.
const ConditionReady = "Ready"

// Phase is the lifecycle phase of an agent resource.
// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Completed;Terminating;Failed
type Phase string

// Lifecycle phases.
const (
	PhasePending      Phase = "Pending"
	PhaseProvisioning Phase = "Provisioning"
	PhaseReady        Phase = "Ready"
	PhaseCompleted    Phase = "Completed"
	PhaseTerminating  Phase = "Terminating"
	PhaseFailed       Phase = "Failed"
)

// BoxName returns the AgentBox and namespace name for an agent box ID.
func BoxName(boxID string) string { return "agent-box-" + boxID }

// Names of the workspace and worker resources, fixed because their namespace is unique per agent box.
const (
	WorkspaceName = "agent-workspace"
	WorkerName    = "agent-worker"
)
