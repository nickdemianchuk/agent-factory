# agent-factory

Kubernetes operator that provisions isolated runtimes for agents.

Each agent session is an `AgentBox` plus the resources that live in it. Every resource in a session shares one fixed `AgentSessionID` (a UUID), carried in `spec.agentSessionID` and the `agentfactory.io/session-id` label.

| Kind | Scope | Role | Kubernetes resources | Name |
| --- | --- | --- | --- | --- |
| `AgentBox` | cluster | isolated agent runtime, parent of the others | Namespace, ServiceAccount, Role, RoleBinding | `box-<uuid>` |
| `AgentWorkspace` | box namespace | agent filesystem, deleted with the box | PersistentVolumeClaim | `workspace-<uuid>` |
| `AgentWorker` | box namespace | agent worker, deleted with the box | Pod mounting `workspace-<uuid>` | `worker-<uuid>` |

Resources are provisioned in that order: a workspace waits for a ready box, and a worker waits for a ready workspace.

## AgentFactory

`AgentFactory` is not a CRD. It is the controller manager (`cmd/agent-factory`) running the three reconcilers, plus the `factory` package, a client with full CRUD over a session:

```go
f := factory.New(c)
s, err := f.Create(ctx, "", factory.Spec{
	Workspace: agentv1.AgentWorkspaceSpec{Size: resource.MustParse("1Gi")},
	Worker:    agentv1.AgentWorkerSpec{Image: "busybox"},
})
s, err = f.Get(ctx, s.ID)
err = f.UpdateWorkspace(ctx, s.ID, func(sp *agentv1.AgentWorkspaceSpec) { sp.Size = resource.MustParse("2Gi") })
err = f.Delete(ctx, s.ID) // deletes the box, and the children with it
```

## Development

```sh
make manifests generate  # CRDs and RBAC into k8s/, deepcopy into api/
make test                # envtest
make lint
make install deploy IMG=<image>
```

See `examples/` for sample manifests and `docs/design.md` for the design.
