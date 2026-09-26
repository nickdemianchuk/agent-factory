package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentWorkerSpec defines the desired state of an agent worker.
type AgentWorkerSpec struct {
	// AgentSessionID is the session this worker belongs to.
	// +required
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="agentSessionID is immutable"
	AgentSessionID string `json:"agentSessionID"`

	// Image is the container image of the worker.
	// +required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Command overrides the image entrypoint.
	// +optional
	// +listType=atomic
	Command []string `json:"command,omitempty"`

	// Args are the arguments to the entrypoint.
	// +optional
	// +listType=atomic
	Args []string `json:"args,omitempty"`

	// Env are environment variables for the worker container.
	// +optional
	// +listType=map
	// +listMapKey=name
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources are the compute resources of the worker container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitzero"`

	// WorkspaceMountPath is where the workspace volume is mounted.
	// +optional
	// +kubebuilder:default=/workspace
	WorkspaceMountPath string `json:"workspaceMountPath,omitempty"`
}

// AgentWorkerStatus defines the observed state of an AgentWorker.
type AgentWorkerStatus struct {
	// Phase is the coarse lifecycle state of the worker.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// PodName is the name of the worker Pod.
	// +optional
	PodName string `json:"podName,omitempty"`

	// ObservedGeneration is the last generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest observations of the worker.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=awk
// +kubebuilder:printcolumn:name="Session",type=string,JSONPath=`.spec.agentSessionID`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AgentWorker runs an agent in a Pod, a child of an AgentBox.
type AgentWorker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec AgentWorkerSpec `json:"spec"`
	// +optional
	Status AgentWorkerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AgentWorkerList contains a list of AgentWorker.
type AgentWorkerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AgentWorker `json:"items"`
}
