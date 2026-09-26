package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentWorkerTemplate is the user-settable part of an AgentWorker.
type AgentWorkerTemplate struct {
	// Container image.
	// +required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Overrides the image entrypoint.
	// +optional
	// +listType=atomic
	Command []string `json:"command,omitempty"`

	// Entrypoint arguments.
	// +optional
	// +listType=atomic
	Args []string `json:"args,omitempty"`

	// Environment variables.
	// +optional
	// +listType=map
	// +listMapKey=name
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Compute resources.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitzero"`

	// Workspace mount path.
	// +optional
	// +kubebuilder:default=/workspace
	WorkspaceMountPath string `json:"workspaceMountPath,omitempty"`
}

// AgentWorkerSpec is the desired state of an AgentWorker.
type AgentWorkerSpec struct {
	// Fixed UUID shared by all resources of the agent box.
	// +required
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="agentBoxID is immutable"
	AgentBoxID string `json:"agentBoxID"`

	AgentWorkerTemplate `json:",inline"`
}

// AgentWorkerStatus is the observed state of an AgentWorker.
type AgentWorkerStatus struct {
	// Lifecycle phase.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Worker Pod name.
	// +optional
	PodName string `json:"podName,omitempty"`

	// Last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Latest observations.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=aworker,categories=agentfactory
// +kubebuilder:printcolumn:name="Agent Box ID",type=string,JSONPath=`.spec.agentBoxID`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AgentWorker is an agent worker Pod, owned by an AgentBox.
type AgentWorker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec AgentWorkerSpec `json:"spec"`
	// +optional
	Status AgentWorkerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AgentWorkerList is a list of AgentWorkers.
type AgentWorkerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AgentWorker `json:"items"`
}
