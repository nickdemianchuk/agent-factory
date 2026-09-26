# Design

## Resources

```
AgentFactory (controller + factory package, not a CRD)
└── AgentBox         cluster-scoped   Namespace agent-box-<uuid>, ServiceAccount, Role, RoleBinding
    ├── AgentWorkspace   namespaced   PVC agent-workspace
    └── AgentWorker      namespaced   Pod agent-worker, mounts agent-workspace
```

`agentSessionID` is immutable and must be a lowercase UUID. The factory generates UUID v7, so names sort by creation time. The box and its namespace are named from it (`agent-box-<uuid>`); the workspace and worker have fixed names inside that namespace (`api/v1alpha1/common.go`). Every resource also carries it in the `agentfactory.io/session-id` label.

## Ordering

The box controller provisions a session in order and holds each step back until the previous one is ready:

1. Namespace, ServiceAccount, Role and RoleBinding.
2. `AgentWorkspace` from `spec.workspace`. It is ready when its PVC is `Bound`, or `Pending` on a `WaitForFirstConsumer` storage class, which only binds once a pod mounts it.
3. `AgentWorker` from `spec.worker`, created only after the workspace is ready.

The box is `Ready` once the worker exists. Until then it is `Provisioning` with the reason on its conditions. The workspace and worker reconcilers only turn their own CR into a PVC or a Pod.

## Ownership

Workspace and worker are controller-owned by the box, and the box namespace is owned by the box. The `agent-factory-box-resources-readonly` admission policy rejects spec changes to them from anyone but the controller. Change the box instead:

- `spec.workspace.size` can grow. The claim is resized only when it is bound and its storage class allows expansion.
- `spec.worker` is immutable, since a Pod spec cannot change in place.

## Deletion

Deleting the box deletes its namespace, which removes everything in it. The box holds a finalizer so the namespace is deleted before the box goes away.

## RBAC

`spec.rules` on the box becomes the `agent` Role in the box namespace, bound to the `agent` ServiceAccount that workers run as. The controller needs `escalate` and `bind` on roles to grant rules it does not hold itself.
