// Package factory provisions and manages agent sessions.
//
// A session is an AgentBox and the AgentWorkspace and AgentWorker that live in
// it, all sharing one AgentSessionID. The factory is not a CRD: it is a client
// with full CRUD over those resources, creating them in dependency order.
package factory

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1 "github.com/nickdemianchuk/agent-factory/api/v1alpha1"
)

// Defaults for how long and how often Create waits for a resource to be ready.
const (
	DefaultReadyTimeout = 2 * time.Minute
	DefaultPollInterval = time.Second
)

// ErrNotFound is returned when a session's box does not exist.
var ErrNotFound = errors.New("agent session not found")

// Factory manages agent sessions.
type Factory struct {
	client client.Client

	// ReadyTimeout bounds how long Create waits for each resource to become ready.
	ReadyTimeout time.Duration
	// PollInterval is how often readiness is checked.
	PollInterval time.Duration
}

// New returns a Factory backed by c. The client's scheme must include the agentfactory.io types.
func New(c client.Client) *Factory {
	return &Factory{client: c, ReadyTimeout: DefaultReadyTimeout, PollInterval: DefaultPollInterval}
}

// NewSessionID returns a fresh AgentSessionID.
func NewSessionID() string { return string(uuid.NewUUID()) }

// Session is an AgentBox and its children.
type Session struct {
	ID        string
	Box       *agentv1.AgentBox
	Workspace *agentv1.AgentWorkspace
	Worker    *agentv1.AgentWorker
}

// Spec describes a session to create. Each part's AgentSessionID is filled in by the factory.
type Spec struct {
	Box       agentv1.AgentBoxSpec
	Workspace agentv1.AgentWorkspaceSpec
	Worker    agentv1.AgentWorkerSpec
}

// Create provisions a session: box first, then workspace, then worker, waiting
// for each to be ready before creating the next. An empty sessionID generates one.
func (f *Factory) Create(ctx context.Context, sessionID string, spec Spec) (*Session, error) {
	if sessionID == "" {
		sessionID = NewSessionID()
	}
	spec.Box.AgentSessionID = sessionID
	spec.Workspace.AgentSessionID = sessionID
	spec.Worker.AgentSessionID = sessionID

	box := &agentv1.AgentBox{
		ObjectMeta: metav1.ObjectMeta{Name: agentv1.BoxName(sessionID), Labels: labels(sessionID)},
		Spec:       spec.Box,
	}
	if err := f.client.Create(ctx, box); err != nil {
		return nil, fmt.Errorf("create box: %w", err)
	}
	if err := f.waitReady(ctx, box, &box.Status.Conditions); err != nil {
		return nil, fmt.Errorf("wait for box: %w", err)
	}

	ns := box.Status.Namespace
	ws := &agentv1.AgentWorkspace{
		ObjectMeta: metav1.ObjectMeta{Name: agentv1.WorkspaceName(sessionID), Namespace: ns, Labels: labels(sessionID)},
		Spec:       spec.Workspace,
	}
	if err := f.client.Create(ctx, ws); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	if err := f.waitReady(ctx, ws, &ws.Status.Conditions); err != nil {
		return nil, fmt.Errorf("wait for workspace: %w", err)
	}

	worker := &agentv1.AgentWorker{
		ObjectMeta: metav1.ObjectMeta{Name: agentv1.WorkerName(sessionID), Namespace: ns, Labels: labels(sessionID)},
		Spec:       spec.Worker,
	}
	if err := f.client.Create(ctx, worker); err != nil {
		return nil, fmt.Errorf("create worker: %w", err)
	}
	return &Session{ID: sessionID, Box: box, Workspace: ws, Worker: worker}, nil
}

// Get returns a session. Children that do not exist yet are nil.
func (f *Factory) Get(ctx context.Context, sessionID string) (*Session, error) {
	box := &agentv1.AgentBox{}
	if err := f.client.Get(ctx, client.ObjectKey{Name: agentv1.BoxName(sessionID)}, box); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	s := &Session{ID: sessionID, Box: box}
	ns := box.Status.Namespace
	if ns == "" {
		return s, nil
	}

	ws := &agentv1.AgentWorkspace{}
	switch err := f.client.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkspaceName(sessionID)}, ws); {
	case err == nil:
		s.Workspace = ws
	case !apierrors.IsNotFound(err):
		return nil, err
	}
	worker := &agentv1.AgentWorker{}
	switch err := f.client.Get(ctx, client.ObjectKey{Namespace: ns, Name: agentv1.WorkerName(sessionID)}, worker); {
	case err == nil:
		s.Worker = worker
	case !apierrors.IsNotFound(err):
		return nil, err
	}
	return s, nil
}

// List returns all sessions.
func (f *Factory) List(ctx context.Context) ([]*Session, error) {
	var boxes agentv1.AgentBoxList
	if err := f.client.List(ctx, &boxes); err != nil {
		return nil, err
	}
	sessions := make([]*Session, 0, len(boxes.Items))
	for _, b := range boxes.Items {
		s, err := f.Get(ctx, b.Spec.AgentSessionID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// UpdateBox changes the mutable parts of a session's box.
func (f *Factory) UpdateBox(ctx context.Context, sessionID string, mutate func(*agentv1.AgentBoxSpec)) error {
	return f.update(ctx, &agentv1.AgentBox{}, client.ObjectKey{Name: agentv1.BoxName(sessionID)},
		func(o client.Object) { mutate(&o.(*agentv1.AgentBox).Spec) })
}

// UpdateWorkspace changes the mutable parts of a session's workspace, such as its size.
func (f *Factory) UpdateWorkspace(
	ctx context.Context, sessionID string, mutate func(*agentv1.AgentWorkspaceSpec),
) error {
	key, err := f.childKey(ctx, sessionID, agentv1.WorkspaceName(sessionID))
	if err != nil {
		return err
	}
	return f.update(ctx, &agentv1.AgentWorkspace{}, key,
		func(o client.Object) { mutate(&o.(*agentv1.AgentWorkspace).Spec) })
}

// UpdateWorker changes the mutable parts of a session's worker.
func (f *Factory) UpdateWorker(ctx context.Context, sessionID string, mutate func(*agentv1.AgentWorkerSpec)) error {
	key, err := f.childKey(ctx, sessionID, agentv1.WorkerName(sessionID))
	if err != nil {
		return err
	}
	return f.update(ctx, &agentv1.AgentWorker{}, key,
		func(o client.Object) { mutate(&o.(*agentv1.AgentWorker).Spec) })
}

// Delete removes a session. Deleting the box removes every child with it.
func (f *Factory) Delete(ctx context.Context, sessionID string) error {
	box := &agentv1.AgentBox{ObjectMeta: metav1.ObjectMeta{Name: agentv1.BoxName(sessionID)}}
	if err := f.client.Delete(ctx, box); err != nil {
		if apierrors.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (f *Factory) childKey(ctx context.Context, sessionID, name string) (client.ObjectKey, error) {
	s, err := f.Get(ctx, sessionID)
	if err != nil {
		return client.ObjectKey{}, err
	}
	if s.Box.Status.Namespace == "" {
		return client.ObjectKey{}, fmt.Errorf("box for session %s is not provisioned", sessionID)
	}
	return client.ObjectKey{Namespace: s.Box.Status.Namespace, Name: name}, nil
}

func (f *Factory) update(
	ctx context.Context, obj client.Object, key client.ObjectKey, mutate func(client.Object),
) error {
	return retryOnConflict(ctx, func() error {
		if err := f.client.Get(ctx, key, obj); err != nil {
			return err
		}
		mutate(obj)
		return f.client.Update(ctx, obj)
	})
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

// waitReady polls obj until its Ready condition is true for the current generation.
func (f *Factory) waitReady(ctx context.Context, obj client.Object, conds *[]metav1.Condition) error {
	poll := func(ctx context.Context) (bool, error) {
		if err := f.client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			// A cached client may not have observed a just-created object yet.
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

func labels(sessionID string) map[string]string {
	return map[string]string{agentv1.SessionIDLabel: sessionID}
}
