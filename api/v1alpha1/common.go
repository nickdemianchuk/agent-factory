package v1alpha1

// SessionIDLabel is set on every resource of a session.
const SessionIDLabel = "agentfactory.io/session-id"

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

// BoxName returns the AgentBox and namespace name for a session.
func BoxName(sessionID string) string { return "agent-box-" + sessionID }

// WorkspaceName returns the AgentWorkspace and PVC name for a session.
func WorkspaceName(sessionID string) string { return "agent-workspace-" + sessionID }

// WorkerName returns the AgentWorker and Pod name for a session.
func WorkerName(sessionID string) string { return "agent-worker-" + sessionID }
