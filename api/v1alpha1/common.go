package v1alpha1

const SessionIDLabel = "agentfactory.io/session-id"

const Finalizer = "agentfactory.io/finalizer"

const ConditionReady = "Ready"

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

func BoxName(sessionID string) string { return "box-" + sessionID }

func WorkspaceName(sessionID string) string { return "workspace-" + sessionID }

func WorkerName(sessionID string) string { return "worker-" + sessionID }
