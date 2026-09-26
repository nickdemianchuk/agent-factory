# agent-factory

Kubernetes operator that provisions isolated runtimes for agents.

Each agent session is one `AgentBox`. Its spec carries the workspace and worker templates, and the box controller provisions the rest. Every resource in a session shares one fixed `AgentSessionID` (a UUID), carried in `spec.agentSessionID` and the `agentfactory.io/session-id` label.

| Kind | Scope | Role | Kubernetes resources | Name |
| --- | --- | --- | --- | --- |
| `AgentBox` | cluster | isolated agent runtime, parent of the others | Namespace, ServiceAccount, Role, RoleBinding | `agent-box-<uuid>` |
| `AgentWorkspace` | box namespace | agent filesystem, created and owned by the box | PersistentVolumeClaim | `agent-workspace-<uuid>` |
| `AgentWorker` | box namespace | agent worker, created and owned by the box | Pod mounting `agent-workspace-<uuid>` | `agent-worker-<uuid>` |

```yaml
apiVersion: agentfactory.io/v1alpha1
kind: AgentBox
metadata:
  name: agent-box-11111111-1111-4111-8111-111111111111
spec:
  agentSessionID: 11111111-1111-4111-8111-111111111111
  workspace:
    size: 1Gi
  worker:
    image: busybox:1.37
```

The box controller creates the namespace and RBAC, then the workspace, and creates the worker once the workspace is ready. Workspaces and workers are read-only for everyone but the controller; change the box instead. Deleting the box deletes everything.

## AgentFactory

`AgentFactory` is not a CRD. It is the controller (`cmd/agent-factory-controller`) running the three reconcilers, plus the `factory` package, a client with full CRUD over a session:

```go
f := factory.New(c)
s, err := f.Create(ctx, "", agentv1.AgentBoxSpec{
	Workspace: agentv1.AgentWorkspaceTemplate{Size: resource.MustParse("1Gi")},
	Worker:    agentv1.AgentWorkerTemplate{Image: "busybox"},
})
s, err = f.Get(ctx, s.ID)
err = f.UpdateWorkspace(ctx, s.ID, func(w *agentv1.AgentWorkspaceTemplate) { w.Size = resource.MustParse("2Gi") })
err = f.Delete(ctx, s.ID)
```

## Development

```sh
make manifests generate  # CRDs and RBAC into k8s/, deepcopy into api/
make test                # envtest
make lint
make install deploy IMG=<image>
```

See `examples/agentbox.yaml` for a sample and `docs/design.md` for the design.
