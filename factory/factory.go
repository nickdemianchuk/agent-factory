// Package factory creates and manages agent boxes, each with its workspace and worker.
package factory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
)

// Defaults for waiting on readiness.
const (
	DefaultReadyTimeout = 2 * time.Minute
	DefaultPollInterval = time.Second
)

// ErrNotFound is returned when an agent box does not exist.
var ErrNotFound = errors.New("agent box not found")

// Factory manages agent boxes.
type Factory struct {
	client client.Client

	ReadyTimeout time.Duration
	PollInterval time.Duration
}

// New returns a Factory backed by c.
func New(c client.Client) *Factory {
	return &Factory{client: c, ReadyTimeout: DefaultReadyTimeout, PollInterval: DefaultPollInterval}
}

// NewAgentBoxID returns a new agent box ID, a time-ordered UUID v7.
func NewAgentBoxID() string { return uuid.Must(uuid.NewV7()).String() }

// AgentBox is an agent box: the box resource with its workspace and worker.
type AgentBox struct {
	ID        string
	Box       *agentv1.AgentBox
	Workspace *agentv1.AgentWorkspace
	Worker    *agentv1.AgentWorker
}

// Create provisions an agent box from its spec, which carries the workspace and worker templates.
// An empty boxID generates one. It returns once the box is ready, meaning its workspace is
// ready and its worker exists.
func (f *Factory) Create(ctx context.Context, boxID string, spec agentv1.AgentBoxSpec) (*AgentBox, error) {
	if boxID == "" {
		boxID = NewAgentBoxID()
	}
	spec.AgentBoxID = boxID

	box := &agentv1.AgentBox{
		ObjectMeta: metav1.ObjectMeta{Name: agentv1.BoxName(boxID), Labels: labels(boxID)},
		Spec:       spec,
	}
	if err := f.client.Create(ctx, box); err != nil {
		return nil, fmt.Errorf("create box: %w", err)
	}
	if err := f.waitReady(ctx, box, &box.Status.Conditions); err != nil {
		return nil, fmt.Errorf("wait for box: %w", err)
	}

	var s *AgentBox
	provisioned := func(ctx context.Context) (bool, error) {
		var err error
		if s, err = f.Get(ctx, boxID); err != nil {
			return false, err
		}
		return s.Workspace != nil && s.Worker != nil, nil
	}
	if err := wait.PollUntilContextTimeout(ctx, f.PollInterval, f.ReadyTimeout, true, provisioned); err != nil {
		return nil, fmt.Errorf("wait for workspace and worker: %w", err)
	}
	return s, nil
}

// Get returns a nil workspace or worker that does not exist yet.
func (f *Factory) Get(ctx context.Context, boxID string) (*AgentBox, error) {
	box := &agentv1.AgentBox{}
	if err := f.client.Get(ctx, client.ObjectKey{Name: agentv1.BoxName(boxID)}, box); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	s := &AgentBox{ID: boxID, Box: box}
	ns := box.Status.Namespace
	if ns == "" {
		return s, nil
	}

	ws := &agentv1.AgentWorkspace{}
	switch err := f.client.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkspaceName}, ws); {
	case err == nil:
		s.Workspace = ws
	case !apierrors.IsNotFound(err):
		return nil, err
	}
	worker := &agentv1.AgentWorker{}
	switch err := f.client.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkerName}, worker); {
	case err == nil:
		s.Worker = worker
	case !apierrors.IsNotFound(err):
		return nil, err
	}
	return s, nil
}

// List returns all agent boxes.
func (f *Factory) List(ctx context.Context) ([]*AgentBox, error) {
	var boxes agentv1.AgentBoxList
	if err := f.client.List(ctx, &boxes); err != nil {
		return nil, err
	}
	agentBoxes := make([]*AgentBox, 0, len(boxes.Items))
	for _, b := range boxes.Items {
		s, err := f.Get(ctx, b.Spec.AgentBoxID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		agentBoxes = append(agentBoxes, s)
	}
	return agentBoxes, nil
}

// UpdateBox mutates the box spec. The box controller propagates the workspace template to the
// workspace; the worker template is immutable.
func (f *Factory) UpdateBox(ctx context.Context, boxID string, mutate func(*agentv1.AgentBoxSpec)) error {
	return retryOnConflict(ctx, func() error {
		box := &agentv1.AgentBox{}
		if err := f.client.Get(ctx, client.ObjectKey{Name: agentv1.BoxName(boxID)}, box); err != nil {
			if apierrors.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		mutate(&box.Spec)
		return f.client.Update(ctx, box)
	})
}

// UpdateWorkspace mutates the workspace template, such as its size.
func (f *Factory) UpdateWorkspace(
	ctx context.Context, boxID string, mutate func(*agentv1.AgentWorkspaceTemplate),
) error {
	return f.UpdateBox(ctx, boxID, func(sp *agentv1.AgentBoxSpec) { mutate(&sp.Workspace) })
}

// Delete removes an agent box, which removes its workspace and worker.
func (f *Factory) Delete(ctx context.Context, boxID string) error {
	box := &agentv1.AgentBox{ObjectMeta: metav1.ObjectMeta{Name: agentv1.BoxName(boxID)}}
	if err := f.client.Delete(ctx, box); err != nil {
		if apierrors.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func retryOnConflict(ctx context.Context, fn func() error) error {
	var err error
	for range 5 {
		if err = fn(); !apierrors.IsConflict(err) {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return err
}

func (f *Factory) waitReady(ctx context.Context, obj client.Object, conds *[]metav1.Condition) error {
	poll := func(ctx context.Context) (bool, error) {
		if err := f.client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			// Cache may not have seen the new object yet.
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		c := meta.FindStatusCondition(*conds, agentv1.ConditionReady)
		if c == nil || c.ObservedGeneration != obj.GetGeneration() {
			return false, nil
		}
		if c.Status == metav1.ConditionFalse && isTerminalReason(c.Reason) {
			return false, fmt.Errorf("%s: %s", c.Reason, c.Message)
		}
		return c.Status == metav1.ConditionTrue, nil
	}
	return wait.PollUntilContextTimeout(ctx, f.PollInterval, f.ReadyTimeout, true, poll)
}

func isTerminalReason(reason string) bool {
	switch reason {
	case "ProvisionFailed", "ClaimFailed", "ClaimLost":
		return true
	}
	return false
}

func labels(boxID string) map[string]string {
	return map[string]string{agentv1.AgentBoxIDLabel: boxID}
}
