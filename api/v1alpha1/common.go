package v1alpha1

// SessionIDLabel is set on every resource that belongs to an agent session.
const SessionIDLabel = "agentfactory.io/session-id"

// Finalizer is set on every agent resource that owns cleanup work.
const Finalizer = "agentfactory.io/finalizer"

// ConditionReady is the condition type reporting whether a resource is usable.
const ConditionReady = "Ready"

// Phase is the coarse lifecycle state of an agent resource.
// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Completed;Terminating;Failed
type Phase string

const (
	PhasePending      Phase = "Pending"
	PhaseProvisioning Phase = "Provisioning"
	PhaseReady        Phase = "Ready"
	PhaseCompleted    Phase = "Completed"
	PhaseTerminating  Phase = "Terminating"
	PhaseFailed       Phase = "Failed"
)

// BoxName returns the name of the AgentBox and its namespace for a session.
func BoxName(sessionID string) string { return "box-" + sessionID }

// WorkspaceName returns the name of the AgentWorkspace and its PVC for a session.
func WorkspaceName(sessionID string) string { return "workspace-" + sessionID }

// WorkerName returns the name of the AgentWorker and its Pod for a session.
func WorkerName(sessionID string) string { return "worker-" + sessionID }
