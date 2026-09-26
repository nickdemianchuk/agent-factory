package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentWorkspaceSpec defines the desired state of an agent filesystem.
type AgentWorkspaceSpec struct {
	// AgentSessionID is the session this workspace belongs to.
	// +required
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="agentSessionID is immutable"
	AgentSessionID string `json:"agentSessionID"`

	// Size is the requested storage capacity. It can only grow.
	// +required
	Size resource.Quantity `json:"size"`

	// StorageClassName is the storage class of the backing volume.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="storageClassName is immutable"
	StorageClassName *string `json:"storageClassName,omitempty"`

	// AccessModes are the access modes of the backing volume. Defaults to ReadWriteOnce.
	// +optional
	// +listType=set
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="accessModes is immutable"
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

// AgentWorkspaceStatus defines the observed state of an AgentWorkspace.
type AgentWorkspaceStatus struct {
	// Phase is the coarse lifecycle state of the workspace.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// ClaimName is the name of the backing PersistentVolumeClaim.
	// +optional
	ClaimName string `json:"claimName,omitempty"`

	// ObservedGeneration is the last generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest observations of the workspace.
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

// AgentWorkspace is the filesystem of an agent, a child of an AgentBox.
type AgentWorkspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec AgentWorkspaceSpec `json:"spec"`
	// +optional
	Status AgentWorkspaceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AgentWorkspaceList contains a list of AgentWorkspace.
type AgentWorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AgentWorkspace `json:"items"`
}
