package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentWorkspaceTemplate is the user-settable part of an AgentWorkspace.
type AgentWorkspaceTemplate struct {
	// Requested capacity; can only grow.
	// +required
	Size resource.Quantity `json:"size"`

	// Storage class of the volume.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="storageClassName is immutable"
	StorageClassName *string `json:"storageClassName,omitempty"`

	// Volume access modes; defaults to ReadWriteOnce.
	// +optional
	// +listType=set
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="accessModes is immutable"
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

// AgentWorkspaceSpec is the desired state of an AgentWorkspace.
type AgentWorkspaceSpec struct {
	// Fixed session UUID shared by all resources in the box.
	// +required
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="agentSessionID is immutable"
	AgentSessionID string `json:"agentSessionID"`

	AgentWorkspaceTemplate `json:",inline"`
}

// AgentWorkspaceStatus is the observed state of an AgentWorkspace.
type AgentWorkspaceStatus struct {
	// Lifecycle phase.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Backing PVC name.
	// +optional
	ClaimName string `json:"claimName,omitempty"`

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
// +kubebuilder:resource:shortName=aws
// +kubebuilder:printcolumn:name="Session",type=string,JSONPath=`.spec.agentSessionID`
// +kubebuilder:printcolumn:name="Size",type=string,JSONPath=`.spec.size`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AgentWorkspace is an agent filesystem, child of an AgentBox.
type AgentWorkspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec AgentWorkspaceSpec `json:"spec"`
	// +optional
	Status AgentWorkspaceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AgentWorkspaceList is a list of AgentWorkspaces.
type AgentWorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AgentWorkspace `json:"items"`
}
