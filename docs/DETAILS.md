# Kubernetes Resource Validator
Detailed Documentation

## Overview
The Kubernetes Resource Validator checks whether Kubernetes resources are valid. The definition of "valid" resources depends on which `validator` implementations are loaded.

This library includes a number of [built-in validators](#built-in-validators). In addition, consumers of this library can provide their own [custom validators](#custom-validators).

Invalid resources are flagged as `violation`s, that are written to a [log](#logging). Consumers of this library can inject any of a number of standard logging implementations.

The Kubernetes Resource Validator can run either [in-cluster](#running-in-kubernetes) or [out-of-cluster](#running-out-of-cluster), such as from a CI environment.

## Motivation
Although alternative tools exist that validate Kubernetes resources, we had a specific need that those tools couldn't satisfy: we needed to recursively search through resource [owner reference](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/#owner-references-in-object-specifications) trees, in order to validate our Kubernetes resources.

The alternatives we evaluated allow defining custom policies declaratively. However, the declarative approach proved restrictive for our purposes, so we opted for a pluggable imperative approach.

## Built-In Validators
This repo contains implementations for several built-in validators:

### Allowed Pods
The [allowed pods validator](./pkg/validators/allowed_pods/) ensures that all running `Pod`s match at least one of the following conditions:
* The `Pod` exists in a predefined configurable `allowlist`
* Any of the `Pod`'s direct owners (see [`ownerReferences`](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/)) is in the `allowlist`
* Any of the `Pod`'s owners' owners (recursively) is in the `allowlist`
* The `Pod` is exempt from validation (see the [Exemptions](#exemptions) section)

Read more about the [`allowlist`](#allowlist-format) format.

### Readiness
The [readiness validator](./pkg/validators/readiness/) verifies that resources listed in a predefined `readinesslist` are ready.

A resource is "ready" if its:
* `status.conditions` field has an entry with `type: Ready` and `status: "True"` fields
* `status.ready` field is `true`

See more about `conditions` [here](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties).

Note that this validator does *not* directly probe for [pod readiness](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/), but rather checks the `status` field.

Read more about the [`readinesslist`](#readinesslist-format) format.

### Freshness
The [freshness validator](./pkg/validators/freshness/) issues violations if the age of any resource is above a certain value (time elapsed since `creationTimestamp`). The age is configurable.

### Privileged Pods
The [privileged pods validator](./pkg/validators/privileged_pods/) issues violations if any pods are running as privileged pods.

## Custom Validators
You can create your own validation logic by implementing the [`Validator`](./pkg/common/types.go) interface.

## Exempt Resources
Certain resources can be exempt from validation. This is done by placing a specific `label` with a specific value on that resource.

The name of that `label` and its value are configurable via the `exempt.labelName` and `exempt.labelValue` configuration fields.

Note that the validator you run must support this feature for the resource to actually be exempt from validation.

## Aborting Validations
Occasionally, the target Kubernetes cluster might yield inconsistent validation results. For example, running pods might not the allowlist during deployment of resources to the cluster.

In this case, you can abort the validation by setting a `ConfigMap` to contain a value, indicating to the Kubernetes Resource Validator to abort the validation.

This `ConfigMap` can be specified in the Kubernetes Resource Validator's configuration:
```yaml
abort:
  configMapNamespace: "default"
  configMapName: "resource-validation-abort"
  configMapField: "deploying"
```

Set the value of the `configMapField` to `"true"` to abort the validation. In all other cases (e.g. the `ConfigMap` is not found) the validation will execute normally.

## Logging
Kubernetes Resource Validator uses the [`logr`](https://github.com/go-logr/logr) library as its logging interface.

The actual logging implementation is set by users of this library. See the `logr` documentation for a partial [list of supported logging implementations](https://github.com/go-logr/logr#implementations-non-exhaustive).

**Note** that the `logr` instance should be placed on the `ctx` parameter of the `validation.NewValidation()` function. For example:

```go
logger := stdr.New(stdlog.New(os.Stderr, "", stdlog.LstdFlags))
ctx = logr.NewContext(ctx, logger)
validationInstance, err := validation.NewValidation(ctx)
```

## Configuration
By default, the configuration directory is located in `/config/`. You can change this by setting the `CONFIG_DIR` environment variable.

The configuration directory may contain the following files:
* `config.yaml` - used for general app configuration, such as `abort` and `exempt` settings. It may also include values for specific validators.
* `additionalResourceTypes.yaml` - used to determine which resource kinds to include in validations.
* `allowlist.yaml` - used by the built-in allowed pods validator.
* `readinesslist.yaml` - used by the built-in readiness validator.

## Resource Kinds
The Kubernetes Resource Validator validates these built-in Kubernetes [workload](https://kubernetes.io/docs/concepts/workloads/) resources:
* `Pod`
* `Deployment`
* `ReplicaSet`
* `StatefulSet`
* `ReplicationController`
* `DaemonSet`
* `Job`
* `CronJob`

In order to support additional resources, you can specify which resources the Kubernetes Resource Validator should handle: Place an `additionalResourceTypes.yaml` file in the default configuration directory.

If you are running in-cluster, you should separately grant the pod running the Kubernetes Resource Validator permissions to read/list these additional resource types (see [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)).

## Running the Application
This repo is a `go` library that is meant to be imported by other `go` applications.

As a convenience, this repo includes a `cmd/sample.go` file that can be compiled and executed as a standalone application for development and evaluations purposes.

### Running in Kubernetes
If you would like to run your resource validator from within a Kubernetes cluster, you must follow these steps:
* Create a `go` application that uses this library. This application:
  * Must define which validators to run
  * Could include custom validators
  * Could include custom post-validation logic
  * Could choose a specific logger implementation
* Create a `Dockerfile` that copies that application.
* Build the `Dockerfile` into an image.
* Push that image to an image registry.
* Create Kubernetes resources that grant access to your application to list and view other Kubernetes resources (e.g. `ClusterRole`, `ClusterRoleBinding` and `ServiceAccount`).
* Create configuration Kubernetes resources (e.g. `Secret`s and/or `ConfigMap`s).
* Create a workload Kubernetes resource (e.g. a `CronJob` or a `Job`) that:
    * References the above image
    * References the above `ServiceAccount`
    * Would mount the configuration as files into the running container
* Deploy the above Kubernetes resources to a cluster (you can deploy them directly, or use [`Helm`](https://helm.sh/), [`Kustomize`](https://kustomize.io/) or similar tools to help you with this).

### Running Out-of-Cluster
By default, the Kubernetes Resource Validator assumes it is running inside a Kubernetes cluster.

However, if you wish to run the Kubernetes Resource Validator from a CI environment, for development purposes, or for any other reason from outside the cluster, follow these steps:
* Set the value of the `KUBECONFIG` environment variable to point to the location of your cluster's `kubeconfig` file.
  
  If the above fails, the Kubernetes Resource Validator will attempt to connect to a cluster based on the `config` file in your homedir's `.kube` directory.
* Set the value of the `CONFIG_DIR` environment variable to point to the location where your configuration files reside (`config.yaml`, `allowlist.yaml`, `readinesslist.yaml`, etc.)

Place the `allowlist.yaml` and `readinesslist.yaml` files in some directory (e.g. `/home/config`) and run the validator application with environment variable `CONFIG_DIR` set to that directory:
  ```sh
  CONFIG_DIR=/home/config ./bin/k8s-resource-validator
  ```

## Building this Project
```sh
make build
```

## Testing
Run tests locally:
```sh
make test
```

View test coverage locally:
```sh
go tool cover -html=cover.out
```

### Addendum
## Allowlist format
The `allowlist.yaml` file looks like this:
```yaml
- name: validator
  namespace: default
  kind: Job
- name: coredns
  namespace: kube-system
  kind: Deployment
```

## Readinesslist format
The `readinesslist.yaml` file looks like this:
```yaml
- name: a-must-be-ready-namespaced-resource-name
  namespace: some-namespace
  kind: Policy
- name: a-must-be-ready-cluster-wide-resource-name
  kind: ClusterPolicy
```
