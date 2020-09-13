# prom-operator

> forked from [sample-controller](https://github.com/kubernetes/sample-controller)

This repository implements a `prom-operator` for watching `CustomMetric` resources as
defined with a CustomResourceDefinition (CRD).

## Details

The `prom-operator` uses [client-go library](https://github.com/kubernetes/client-go/tree/master/tools/cache) extensively.

The `CustomMetric` resource creates and manages a `prometheus` server, with a `stackdriver-prometheus-sidecar`, that scrapes the metric(s) you defined and send them to GCP Cloud Monitoring API.

It makes use of the generators in [k8s.io/code-generator](https://github.com/kubernetes/code-generator)
to generate a typed client, informers, listers and deep-copy functions. You can
do this yourself using the `./hack/update-codegen.sh` script.

## Code

`CustomMetric` resource is defined in `pkg/apis/promoperator/v1alpha1/types.go`.

Code regeneration is needed when any changes is made to the resource definition.

```sh
./hack/update-codegen.sh
```

The `update-codegen` script will automatically generate the following files &
directories:

* `pkg/apis/promoperator/v1alpha1/zz_generated.deepcopy.go`
* `pkg/generated/`

Changes should not be made to these files manually, and when creating your own
controller based off of this implementation you should not copy these files and
instead run the `update-codegen` script to generate your own.

**Note:** If you intend to [generate code](#changes-to-the-types) then you will also need the
code-generator repo to exist in an old-style location.  One easy way to do this is to use the command `go mod vendor` to create and populate the `vendor` directory.

## Build

```sh
# build with GCP Cloud Build
make build

# build local
# Note: tagged with `:local` by default
make build-local

# build local and push to dockerhub
make build-dockerhub
```

## Deploy

**Prerequisite**: Since the prom-operator uses `apps/v1` deployments, the Kubernetes cluster version should be greater than 1.9.

```sh
# create the CRD, spin up the prom-operator and create a demo CR
cd ./artifacts/examples
./deploy.sh
```

## Teardown

```sh
cd ./artifacts/examples
./remove.sh
```
