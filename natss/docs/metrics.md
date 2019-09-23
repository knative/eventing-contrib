# Metrics

Follow the instructions on Knative eventing to
[access Prometheus metrics](https://github.com/knative/eventing/blob/master/docs/metrics.md#access-metrics).
Then do the following to access NATS Streaming metrics.

## Add NatssChannel scrape jobs in the Prometheus config map

Edit the config map **prometheus-scrape-config**:

```bash
kubectl edit cm -n knative-monitoring prometheus-scrape-config
```

Add the following job entries under
**data**.**prometheus.yml**.**scrape_configs**:

```yaml
# natsschannel_controller
- job_name: natsschannel_controller
  scrape_interval: 3s
  scrape_timeout: 3s
  kubernetes_sd_configs:
    - role: pod
  relabel_configs:
    # Scrape only the the targets matching the following metadata
    - source_labels:
        [
          __meta_kubernetes_namespace,
          __meta_kubernetes_pod_label_messaging_knative_dev_role,
          __meta_kubernetes_pod_label_messaging_knative_dev_channel,
          __meta_kubernetes_pod_container_port_name,
        ]
      action: keep
      regex: knative-eventing;controller;natss-channel;metrics
    # Rename metadata labels to be reader friendly
    - source_labels: [__meta_kubernetes_namespace]
      target_label: namespace
    - source_labels: [__meta_kubernetes_pod_name]
      target_label: pod
    - source_labels: [__meta_kubernetes_service_name]
      target_label: service

# natsschannel_dispatcher
- job_name: natsschannel_dispatcher
  scrape_interval: 3s
  scrape_timeout: 3s
  kubernetes_sd_configs:
    - role: pod
  relabel_configs:
    # Scrape only the the targets matching the following metadata
    - source_labels:
        [
          __meta_kubernetes_namespace,
          __meta_kubernetes_pod_label_messaging_knative_dev_role,
          __meta_kubernetes_pod_label_messaging_knative_dev_channel,
          __meta_kubernetes_pod_container_port_name,
        ]
      action: keep
      regex: knative-eventing;dispatcher;natss-channel;metrics
    # Rename metadata labels to be reader friendly
    - source_labels: [__meta_kubernetes_namespace]
      target_label: namespace
    - source_labels: [__meta_kubernetes_pod_name]
      target_label: pod
    - source_labels: [__meta_kubernetes_service_name]
      target_label: service
```

## Restart Prometheus pods to pick up this new config map changes

```bash
kubectl delete pods -n knative-monitoring prometheus-system-0 prometheus-system-1
```

## Check Prometheus jobs

Do a port-forward to access Prometheus targets endpoint:

```bash
kubectl port-forward -n knative-monitoring \
   $(kubectl get pods -n knative-monitoring \
   --selector=app=prometheus --output=jsonpath="{.items[0].metadata.name}") \
   9090
```

Access Prometheus targets endpoint
[http://localhost:9090/targets](http://localhost:9090/targets). Both the
**natsschannel_controller** and **natsschannel_dispatcher** jobs should be up.

## Check NatssChannel controller metrics

Do a port-forward to access NatssChannel controller metrics endpoint:

```bash
kubectl port-forward -n knative-eventing \
   $(kubectl get pods -n knative-eventing \
   --selector='messaging.knative.dev/channel=natss-channel,messaging.knative.dev/role=controller' -o=jsonpath='{.items[0].metadata.name}') \
   9091:9090
```

Create a test NatssChannel:

```bash
cat << EOF | kubectl apply -f -
apiVersion: messaging.knative.dev/v1alpha1
kind: NatssChannel
metadata:
  name: my-test-channel
  namespace: default
EOF
```

Access NatssChannel controller metrics
[http://localhost:9091/metrics](http://localhost:9091/metrics).
