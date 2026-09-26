package controllers

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
)

func sessionLabels(sessionID string) map[string]string {
	return map[string]string{agentv1.SessionIDLabel: sessionID}
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

func getBox(ctx context.Context, c client.Client, sessionID string) (*agentv1.AgentBox, error) {
	var box agentv1.AgentBox
	if err := c.Get(ctx, client.ObjectKey{Name: agentv1.BoxName(sessionID)}, &box); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return &box, nil
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
