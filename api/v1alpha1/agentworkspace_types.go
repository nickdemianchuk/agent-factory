package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AgentWorkspaceSpec struct {
	// +required
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="agentSessionID is immutable"
	AgentSessionID string `json:"agentSessionID"`

	// Can only grow.
	// +required
	Size resource.Quantity `json:"size"`

	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="storageClassName is immutable"
	StorageClassName *string `json:"storageClassName,omitempty"`

	// Defaults to ReadWriteOnce.
	// +optional
	// +listType=set
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="accessModes is immutable"
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

type AgentWorkspaceStatus struct {
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// +optional
	ClaimName string `json:"claimName,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=aws
// +kubebuilder:printcolumn:name="Session",type=string,JSONPath=`.spec.agentSessionID`
// +kubebuilder:printcolumn:name="Size",type=string,JSONPath=`.spec.size`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type AgentWorkspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec AgentWorkspaceSpec `json:"spec"`
	// +optional
	Status AgentWorkspaceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

type AgentWorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AgentWorkspace `json:"items"`
}
