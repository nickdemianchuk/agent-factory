package controllers

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
)

const (
	managedByLabel = "app.kubernetes.io/managed-by"
	componentLabel = "app.kubernetes.io/component"
	managedByValue = "agent-factory-controller"

	componentBox       = "box"
	componentWorkspace = "workspace"
	componentWorker    = "worker"
)

func resourceLabels(sessionID, component string) map[string]string {
	return map[string]string{
		agentv1.SessionIDLabel: sessionID,
		managedByLabel:         managedByValue,
		componentLabel:         component,
	}
}

func setReady(conds *[]metav1.Condition, generation int64, ready bool, reason, message string) {
	status := metav1.ConditionFalse
	if ready {
		status = metav1.ConditionTrue
	}
	meta.SetStatusCondition(conds, metav1.Condition{
		Type:               agentv1.ConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	})
}

func isReady(conds []metav1.Condition, generation int64) bool {
	c := meta.FindStatusCondition(conds, agentv1.ConditionReady)
	return c != nil && c.Status == metav1.ConditionTrue && c.ObservedGeneration == generation
}

func ignoreAlreadyExists(err error) error {
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}
