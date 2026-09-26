# Design

## Resources

```
AgentFactory (controller manager + factory package, not a CRD)
└── AgentBox         cluster-scoped   Namespace box-<uuid>, ServiceAccount, Role, RoleBinding
    ├── AgentWorkspace   namespaced   PVC workspace-<uuid>
    └── AgentWorker      namespaced   Pod worker-<uuid>, mounts workspace-<uuid>
```

`agentSessionID` is immutable and must be a lowercase UUID. All names derive from it (`api/v1alpha1/common.go`).

## Ordering

1. `AgentBox` is ready once its namespace and RBAC exist.
2. `AgentWorkspace` stays `Pending` until the box is ready, then creates its PVC. It is ready as soon as the claim exists and is not lost; a claim with `WaitForFirstConsumer` binding only binds once the worker mounts it.
3. `AgentWorker` stays `Pending` until the box and workspace are ready, then creates its Pod.

The factory creates the CRs in this order, waiting for readiness in between. The reconcilers enforce the same order independently.

## Deletion

Workspace and worker carry an owner reference to the box, and the box namespace is owned by the box. Deleting the box deletes the namespace, which removes everything in it. The box holds a finalizer so the namespace is deleted before the box goes away.

## RBAC

`spec.rules` on the box becomes the `agent` Role in the box namespace, bound to the `agent` ServiceAccount that workers run as. The controller needs `escalate` and `bind` on roles to grant rules it does not hold itself.
