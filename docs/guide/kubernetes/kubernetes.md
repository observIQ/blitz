## Kubernetes: highly scaled Blitz with sidecar collectors

This guide shows a simple pattern for running Blitz at scale in Kubernetes:

- **Each Pod runs several Blitz containers**
- **Each Blitz container sends OTLP to a dedicated OpenTelemetry Collector sidecar over `localhost`**
- **Each collector is configured by a ConfigMap and exports using the `debug` exporter**
- **Each collector binds to unique TCP ports to avoid port collision**

### How the ConfigMaps and Deployment work together

- **Collector config**: Each collector sidecar listens on a unique OTLP/gRPC port (4317–4324) and Prometheus (own metrics) port. There ports are defined in the collector config.
- **ConfigMap → volume**: Each collector mounts its config from a ConfigMap via `volumes[].configMap` + `volumeMounts[].subPath`.
- **Blitz → localhost**: Each Blitz container is configured to send to `127.0.0.1:<port>`, matching its paired collector.

In one Pod, the pairs look like this:

- **`blitz-1` → `otel-collector-1`** on `127.0.0.1:4317`
- **`blitz-2` → `otel-collector-2`** on `127.0.0.1:4318`
- …
- **`blitz-8` → `otel-collector-8`** on `127.0.0.1:4324`

### Complete example (recommended)

Use the full, ready-to-apply manifests:

- [`assets/configmap.yaml`](./assets/configmap.yaml)
- [`assets/deployment.yaml`](./assets/deployment.yaml)

From the repo root:

```bash
kubectl apply -f docs/guide/kubernetes/assets/configmap.yaml
kubectl apply -f docs/guide/kubernetes/assets/deployment.yaml
```

```bash
kubectl get pods -l app=blitz
kubectl logs -l app=blitz -c otel-collector-1 --tail=50
```

### Scaling

- **Scale Pods**: `kubectl scale deployment/blitz --replicas=<n>`
- **Adjust per-container rate**: change `--generator-*-rate` (each Blitz container runs independently)
