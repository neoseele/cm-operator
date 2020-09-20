# cm-operator

> forked from [sample-controller](https://github.com/kubernetes/sample-controller)

This repository implements a `cm-operator` for watching `CustomMetric` resources as
defined with a CustomResourceDefinition (CRD).

## Details

The `cm-operator` uses [client-go library](https://github.com/kubernetes/client-go/tree/master/tools/cache) extensively.

The `CustomMetric` resource creates and manages a `prometheus` server, with a `stackdriver-prometheus-sidecar`, that scrapes the metric(s) you defined and send them to GCP Cloud Monitoring API.

It makes use of the generators in [k8s.io/code-generator](https://github.com/kubernetes/code-generator)
to generate a typed client, informers, listers and deep-copy functions. You can
do this yourself using the `./hack/update-codegen.sh` script.

## Code

`CustomMetric` resource is defined in `pkg/apis/cmoperator/v1alpha1/types.go`.

Code regeneration is needed when any changes is made to the resource definition.

```sh
./hack/update-codegen.sh
```

The `update-codegen` script will automatically generate the following files &
directories:

* `pkg/apis/cmoperator/v1alpha1/zz_generated.deepcopy.go`
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

**Prerequisite**: Since the cm-operator uses `apps/v1` deployments, the Kubernetes cluster version should be greater than 1.9.

```sh
# create the CRD, spin up the cm-operator and create a demo CR
make deploy
```

## Teardown

```sh
make teardown
```

## Usage

### Annotate the pod that needs to be scraped

* `cmoperator.io/scrape` (required)
* `cmoperator.io/port` (optional, default: `80`)
* `cmoperator.io/path` (optional, default: `/metrics`)

Example:

```sh
POD=some_pod
# create
kubectl annotate --overwrite pods $POD 'cmoperator.io/scrape'='true' 'cmoperator.io/port'='9990'
# remove
kubectl annotate --overwrite pods $POD 'cmoperator.io/scrape-' 'cmoperator.io/port-'
```

### Annotate the node that needs to be scrapes

> port/path is hardcoded to cadvisor endpoint `:10255/metrics/cadvisor`

* `cmoperator.io/scrape` (required)

Example:

```sh
NODE=some_node
# create
kubectl annotate --overwrite nodes $NODE 'cmoperator.io/scrape'='true'
# remove
kubectl annotate --overwrite nodes $NODE 'cmoperator.io/scrape-'
```

### Create the CR

The listed metrics will be sent to Cloud Monitoring

```yaml
apiVersion: cmoperator.k8s.io/v1alpha1
kind: CustomMetric
metadata:
  name: cm
spec:
  project: nmiu-play
  cluster: ebpf
  location: australia-southeast1-a
  metrics:
    - cilium_*
    - container_network_*
```
