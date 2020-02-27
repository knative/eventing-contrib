# Metrics

Follow the instructions on Knative eventing to
[access Prometheus metrics](https://github.com/knative/eventing/blob/master/docs/metrics.md#access-metrics).
Then do the following to access Apache Kafka metrics.

## Add KafkaChannel scrape jobs in the Prometheus config map

Edit the config map **prometheus-scrape-config**:

```bash
kubectl edit cm -n knative-monitoring prometheus-scrape-config
```

Add the following job entries under
**data**.**prometheus.yml**.**scrape_configs**:

```yaml
# kafkachannel-controller
- job_name: kafkachannel-controller
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
      regex: knative-eventing;controller;kafka-channel;metrics
    # Rename metadata labels to be reader friendly
    - source_labels: [__meta_kubernetes_namespace]
      target_label: namespace
    - source_labels: [__meta_kubernetes_pod_name]
      target_label: pod
    - source_labels: [__meta_kubernetes_service_name]
      target_label: service

# kafkachannel-dispatcher
- job_name: kafkachannel-dispatcher
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
      regex: knative-eventing;dispatcher;kafka-channel;metrics
    # Rename metadata labels to be reader friendly
    - source_labels: [__meta_kubernetes_namespace]
      target_label: namespace
    - source_labels: [__meta_kubernetes_pod_name]
      target_label: pod
    - source_labels: [__meta_kubernetes_service_name]
      target_label: service

# kafkachannel-webhook
- job_name: kafkachannel-webhook
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
      regex: knative-eventing;controller;kafka-channel;metrics
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
[http://localhost:9090/targets](http://localhost:9090/targets). Now, the
**kafkachannel-controller**, **kafkachannel-dispatcher** and
**kafkachannel-webhook** jobs should be up.

## Check KafkaChannel controller metrics

Do a port-forward to access KafkaChannel controller metrics endpoint:

```bash
kubectl port-forward -n knative-eventing \
   $(kubectl get pods -n knative-eventing \
   --selector='messaging.knative.dev/channel=kafka-channel,messaging.knative.dev/role=controller' -o=jsonpath='{.items[0].metadata.name}') \
   9091:9090
```

Create a test KafkaChannel:

```bash
cat << EOF | kubectl apply -f -
apiVersion: messaging.knative.dev/v1alpha1
kind: KafkaChannel
metadata:
  name: my-test-channel
  namespace: default
EOF
```

Access KafkaChannel controller metrics
[http://localhost:9091/metrics](http://localhost:9091/metrics).
