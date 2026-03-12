---
title: RBAC Aggregation for Telemetry Resources
status: Proposed
date: 2026-03-04
---

# 33. RBAC Aggregation for Telemetry Resources

## Context

The Telemetry module currently lacks Kubernetes-native RBAC aggregation support. Users who are bound to standard Kubernetes roles (`admin`, `edit`, `view`) cannot manage telemetry resources without manual role configuration. 
This creates operational friction and deviates from Kubernetes best practices where modules typically integrate with the built-in role hierarchy.

### Direct Resources Managed by Telemetry Module

The Telemetry module directly owns and manages the following **cluster-scoped** resources:

**Pipeline Resources (v1beta1):**
- `logpipelines.telemetry.kyma-project.io` - Configure log collection and forwarding
- `metricpipelines.telemetry.kyma-project.io` - Configure metric collection and forwarding  
- `tracepipelines.telemetry.kyma-project.io` - Configure trace collection and forwarding

**Operator Resources (v1beta1):**
- `telemetries.operator.kyma-project.io` - Manage the lifecycle and configuration of the Telemetry module itself

**Namespace-Scoped Resources:**
- ConfigMap `telemetry-logpipelines` in `kyma-system` - Busola UI for configuring log pipelines
- ConfigMap `telemetry-metricpipelines` in `kyma-system` - Busola UI for configuring metric pipelines
- ConfigMap `telemetry-tracepipelines` in `kyma-system` - Busola UI for configuring trace pipelines

Each resource includes:
- Main resource (spec and metadata)
- Status subresource (health and configuration state)
- Finalizers (cleanup coordination)

**Note:** Reading status is implicit in main resource read access. Writing status and managing finalizers are implicit in main resource update access.

### Indirect Dependencies (Out of Scope)

Telemetry pipelines reference the following resources, which other modules or operators manage:

**Istio Resources:**
- `telemetry.istio.io` - Istio Telemetry API for enabling traces and access logs (managed by Istio operator)

**Service Catalog Resources:**
- `serviceinstances.services.cloud.sap.com` - Cloud Logging Service instances (managed by Service Catalog)
- `servicebindings.services.cloud.sap.com` - Bindings to Cloud Logging Service instances (managed by Service Catalog)

**Secret Resources:**
- Secrets containing credentials for external backends (managed separately, not granted direct access)

**The respective modules handle RBAC for indirect dependencies.** This ADR focuses only on direct Telemetry module resources.

### Kubernetes RBAC Aggregation

Kubernetes provides a mechanism for extending built-in roles through **ClusterRole aggregation**. ClusterRoles with specific labels automatically merge their rules into standard roles:

```yaml
metadata:
  labels:
    rbac.authorization.k8s.io/aggregate-to-view: "true"
    rbac.authorization.k8s.io/aggregate-to-edit: "true"
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
```

With this mechanism, modules integrate with the existing RBAC hierarchy, and users don't need to create custom roles.

#### Standard Kubernetes Roles

Kubernetes defines four default [ClusterRoles](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#user-facing-roles) that form a hierarchy of increasing privileges:

|       Role        |                                                                              Permissions                                                                              | Use Case                                                         |
|:-----------------:|:---------------------------------------------------------------------------------------------------------------------------------------------------------------------:|------------------------------------------------------------------|
|     **view**      |                                 Read-only access to most resources in a namespace. <br/> Cannot view Roles, RoleBindings, or Secrets                                  | Monitoring, debugging, audit (SREs, developers, auditors)        |
|     **edit**      |                       Read/write access to most resources. <br/> Cannot view or modify Roles/RoleBindings <br/> Cannot access Secrets directly                        | Application developers, DevOps engineers managing configurations |
|     **admin**     | Read/write access within a namespace. <br/> Can create/modify Roles and RoleBindings within the namespace <br/> Cannot modify resource quotas or the namespace itself | Namespace administrators managing access within their scope      |
| **cluster-admin** |                                                            Full control over every resource in the cluster                                                            | Platform administrators, cluster operators                       |

**Key Distinction**: The primary difference between `edit` and `admin` is the ability to manage **RBAC resources (Roles/RoleBindings)** within a namespace.

#### Namespace-Scoped vs Cluster-Scoped Access

The **binding type** determines where permissions apply, independent of the role type:

**RoleBinding (Namespace-Scoped):**
```yaml
kind: RoleBinding
metadata:
  name: alice-edit
  namespace: team-a
subjects:
- kind: User
  name: alice
roleRef:
  kind: ClusterRole
  name: edit  # Standard edit role
```
Grants `edit` permissions **only within the `team-a` namespace**  
**Cannot grant access to cluster-scoped resources** (like our telemetry pipelines)

**ClusterRoleBinding (Cluster-Wide):**
```yaml
kind: ClusterRoleBinding
metadata:
  name: bob-edit
subjects:
- kind: User
  name: bob
roleRef:
  kind: ClusterRole
  name: edit  # Standard edit role
``` 
Grants `edit` permissions **cluster-wide** (all namespaces + cluster-scoped resources)

#### Implications for Telemetry Resources

Because `logpipelines`, `metricpipelines`, `tracepipelines`, and `telemetries` are **cluster-scoped resources**, the following constraints apply:

- To grant access to cluster-scoped telemetry resources, you must use a **ClusterRoleBinding**. A namespace-scoped RoleBinding has no effect.
- ️ Users need cluster-level permissions, typically granted to platform/SRE teams
- The ConfigMap `sap-cloud-logging` in the `kube-public` namespace is an exception. Because it is a namespaced resource, you can manage access to it with a RoleBinding in that namespace.

**Important**: Even when telemetry resources aggregate into `edit` or `admin` roles, users still need a **ClusterRoleBinding** to apply those permissions to the cluster-scoped resources.

## Requirements

Based on [Kyma RBAC Decision Record](https://github.com/kyma-project/community/issues/1014) and [Issue #3022](https://github.com/kyma-project/telemetry-manager/issues/3022), the implementation must provide the following capabilities:

1. **View Role**: Read-only access for monitoring and observability
2. **Edit Role**: Full CRUD access for managing telemetry pipelines
3. **Security**: No direct access to Secrets (credentials managed separately)

## Proposal

Create aggregated ClusterRoles with consolidated rules. Status read access is implicit with main resource read access. 
Status write and finalizer access is implicit with main resource update access.


**Example:**
```yaml
rules:
  - apiGroups: ["telemetry.kyma-project.io"]
    resources: ["logpipelines"]
    verbs: ["get", "list", "watch"]  # Includes status read automatically
```

## Decision

We create **two aggregated ClusterRoles** for telemetry resources, following Kubernetes RBAC best practices:

### Role Definitions

|        Role Name        |  Aggregates To  |                                                                                                                           Resources                                                                                                                           |                                       Verbs                                       |                                                                                Rationale                                                                                |
|:-----------------------:|:---------------:|:-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------:|:---------------------------------------------------------------------------------:|:-----------------------------------------------------------------------------------------------------------------------------------------------------------------------:|
| **kyma-telemetry-view** |     `view`      | **Cluster-scoped:**<br/>• `logpipelines`<br/>• `metricpipelines`<br/>• `tracepipelines`<br/>• `telemetries`<br/><br/>**Namespace-scoped:**<br/>• ConfigMap `telemetry-logpipelines`, `telemetry-tracepipelines`, `telemetry-metricpipelines` in `kyma-system` |                              `get`, `list`, `watch`                               |   Read-only access for monitoring, debugging, and observability. Enables SREs, auditors, and developers to view telemetry configurations without modification rights.   |
| **kyma-telemetry-edit** | `edit`, `admin` |                                                                       **Cluster-scoped:**<br/>• `logpipelines`<br/>• `metricpipelines`<br/>• `tracepipelines`<br/>• `telemetries`<br/>                                                                        | `create`, `delete`, `deletecollection`, `get`, `list`, `patch`, `update`, `watch` | Full CRUD access for platform engineers and DevOps teams managing telemetry infrastructure. Does **not** include direct Secret access (credentials managed separately). |

**Note:** We do **not** create a separate `kyma-telemetry-admin` role because:
- The traditional `admin` vs `edit` distinction (managing Roles/RoleBindings) does not apply to cluster-scoped resources
- Both would have identical permissions on telemetry resources
- We intentionally exclude Secret access for security. Separate RBAC policies must manage credentials.

### Binding Requirements

- Because most telemetry resources are **cluster-scoped**, users need **ClusterRoleBindings** to access them.
- For the namespace-scoped ConfigMap `telemetry-logpipelines`, `telemetry-tracepipelines`, `telemetry-metricpipelines`, a RoleBinding in the `kyma-system` namespace is sufficient.

### Testing and Validation

To verify permissions for both cluster-scoped and namespace-scoped resources, we test the roles using `kubectl auth can-i` . To validate that aggregation works correctly, we check the effective permissions of the standard `view` and `edit` roles after aggregation.


