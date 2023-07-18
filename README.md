# node-disruption-controller

Node-disruption-controller is a, as it name implies, a way to control node disruption in a
Kubernetes cluster.

## Description

The main use case of the node disruption controller is to perform impacting maintenance.
Typically a maintenance requires draining the pods of a node then having the node unavailble
for a period of time (maybe forever).

The system is build around a contract: before doing anything on a node, a node disruption
need to be accepted for the node.

The controller is reponsible for accepting node disruption. It does that by looking at
contraints provided by the disruptions bugdets. The application disruption budget serve to
reprensent the constraints of an application running on top of Kubernetes. The main difference
with a PDB is that the Application Disruption Budget can:
- Target PVC, to prevent maintenance even when no pods running. This is useful when relying
  on local storage.
- Can perform a synchronous service level health check. PDB is only looking at the readiness probes


### NodeDisruption

The NodeDisruption represent the disruption of one or more nodes. The controller
doesn't make any asumption on the nature of the disruption (reboot, network down).

```
apiVersion: nodedisruption.criteo.com/v1alpha1
kind: NodeDisruption
metadata:
  labels:
    app.kubernetes.io/name: nodedisruption
    app.kubernetes.io/instance: nodedisruption-sample
    app.kubernetes.io/part-of: node-disruption-controller
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/created-by: node-disruption-controller
  name: nodedisruption-sample
spec:
  nodeSelector: # Select all the nodes impacted by the disruption
    matchLabels:
      kubernetes.io/hostname: fakehostname
```

The controller will change the state: 
                        -> accepted
pending -> processing /
                      \
                        -> rejected

* Pending: the disruption has not been processed yet by the controller
* Processing: the controller is processing the disruption, checking if it can be accepted or not
* Accepted: the disruption has been accepted. the selected nodes can be disrupted. It should be deleted once the disruption is over.
* Rejected: the disruption has been rejected with a reason in the events, it can be safely deleted


### ApplicationDisruptionBudget

The ApplicationDisruptionBudget provide a way to set constraints for an application
running inside Kubernetes. It can select Pod (like PDB) but also PVC (to protect data).


```
apiVersion: nodedisruption.criteo.com/v1alpha1
kind: ApplicationDisruptionBudget
metadata:
  labels:
    app.kubernetes.io/name: applicationdisruptionbudget
    app.kubernetes.io/instance: applicationdisruptionbudget-sample
    app.kubernetes.io/part-of: node-disruption-controller
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/created-by: node-disruption-controller
  name: applicationdisruptionbudget-sample
spec:
  podSelector: # Select pods to protect by the budget
    matchLabels:
      app: nginx
```

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/node-disruption-controller:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/node-disruption-controller:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

