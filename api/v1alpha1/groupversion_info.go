// Package v1alpha1 contains API Schema definitions for the agentfactory.io v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=agentfactory.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is the group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "agentfactory.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&AgentBox{}, &AgentBoxList{},
		&AgentWorkspace{}, &AgentWorkspaceList{},
		&AgentWorker{}, &AgentWorkerList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}
