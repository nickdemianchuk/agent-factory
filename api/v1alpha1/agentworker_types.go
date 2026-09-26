package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AgentWorkerSpec struct {
	// +required
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="agentSessionID is immutable"
	AgentSessionID string `json:"agentSessionID"`

	// +required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// +optional
	// +listType=atomic
	Command []string `json:"command,omitempty"`

	// +optional
	// +listType=atomic
	Args []string `json:"args,omitempty"`

	// +optional
	// +listType=map
	// +listMapKey=name
	Env []corev1.EnvVar `json:"env,omitempty"`

	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitzero"`

	// +optional
	// +kubebuilder:default=/workspace
	WorkspaceMountPath string `json:"workspaceMountPath,omitempty"`
}

type AgentWorkerStatus struct {
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// +optional
	PodName string `json:"podName,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

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

type AgentWorker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec AgentWorkerSpec `json:"spec"`
	// +optional
	Status AgentWorkerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

type AgentWorkerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AgentWorker `json:"items"`
}
