package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Desired state of an AgentBox.
type AgentBoxSpec struct {
	// Fixed session UUID shared by all resources in the box.
	// +required
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="agentSessionID is immutable"
	AgentSessionID string `json:"agentSessionID"`

	// RBAC rules for the box agent.
	// +optional
	// +listType=atomic
	Rules []rbacv1.PolicyRule `json:"rules,omitempty"`
}

// Observed state of an AgentBox.
type AgentBoxStatus struct {
	// Lifecycle phase.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Namespace provisioned for the box.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Service account agents run as.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

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
// +kubebuilder:resource:scope=Cluster,shortName=abox
// +kubebuilder:printcolumn:name="Session",type=string,JSONPath=`.spec.agentSessionID`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Namespace",type=string,JSONPath=`.status.namespace`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Isolated agent runtime; parent of all other agent resources.
type AgentBox struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec AgentBoxSpec `json:"spec"`
	// +optional
	Status AgentBoxStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

type AgentBoxList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AgentBox `json:"items"`
}
