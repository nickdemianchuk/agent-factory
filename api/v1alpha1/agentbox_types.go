package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentBoxSpec defines the desired state of an isolated agent runtime.
type AgentBoxSpec struct {
	// AgentSessionID is the fixed identifier shared by every resource in the box.
	// +required
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="agentSessionID is immutable"
	AgentSessionID string `json:"agentSessionID"`

	// Rules are the RBAC rules granted to the box's agent inside its namespace.
	// +optional
	// +listType=atomic
	Rules []rbacv1.PolicyRule `json:"rules,omitempty"`
}

// AgentBoxStatus defines the observed state of an AgentBox.
type AgentBoxStatus struct {
	// Phase is the coarse lifecycle state of the box.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Namespace is the namespace provisioned for the box.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// ServiceAccountName is the service account agents in the box run as.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// ObservedGeneration is the last generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest observations of the box.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=abox
// +kubebuilder:printcolumn:name="Session",type=string,JSONPath=`.spec.agentSessionID`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Namespace",type=string,JSONPath=`.status.namespace`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AgentBox is an isolated agent runtime and the parent of all other agent resources.
type AgentBox struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec AgentBoxSpec `json:"spec"`
	// +optional
	Status AgentBoxStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AgentBoxList contains a list of AgentBox.
type AgentBoxList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AgentBox `json:"items"`
}
