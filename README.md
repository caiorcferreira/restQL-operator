# restQL Operator

> **Note**: This is an experimental project and is not production ready.

restQL Operator provides a simple and declarative way of provision [restQL]() on a Kubernetes Cluster.

This project provides three custom resources:
- `RestQL`: supports a standard Kubernetes deployment with additional settings like the base YAML configuration and environment tenant. 
- `Query`: allows the definition of a restQL Query Set with a namespace, name and a list of revisions.
- `TenantMapping`: allows the definition of a restQL Tenant with a dictionary of mappings.

### Improvements

- When a `Query` or `TenantMapping` is changed, the restQL Operator triggers a restart of all deployments in order to pick-up the changes. If restQL had a watching mechanism for its YAML configuration, this would not be needed.
- Avoid patches when `Query` or `TenantMapping` does not change.
- Support for backup routines of `Query` and `TenantMapping`.
- Support restart policies like avoiding restarting all deployments at the same time.
- Code and design refactoring. 