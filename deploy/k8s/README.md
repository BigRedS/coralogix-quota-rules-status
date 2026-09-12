# Kubernetes manifests

A minimal kustomize base for the web UI: a `Deployment` pinned to the
`latest` tag and a `ClusterIP` `Service` on port 8765. 

Runs non-root, read-only root filesystem, no capabilities.

You will need to set:
* Namespace
* Ingress etc., however you want to reach this
* Image tag version if you don't trust my `latest` :) 

## Use directly

```sh
kubectl apply -k .
```

## Use as a remote base from another repo

```yaml
# kustomization.yaml
resources:
  - github.com/BigRedS/coralogix-quota-rules-status/deploy/k8s?ref=main
```
