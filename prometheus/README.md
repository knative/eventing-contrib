# Prometheus Event Source for Knative Eventing

The Prometheus Event Source enables Knative Eventing integration with the
[Prometheus Monitoring System](https://prometheus.io/).

## Instant and Range Queries

The Prometheus Event Source supports two kinds of PromQL queries - instant and
range. The _promQL_ property is the basis of a range query if the _step_
property is specified and an instant query otherwise. An instant query returns a
snapshot of the corresponding data stream at the moment when the query executes
on the server. A range query specifies a time interval and a resolution step and
returns a series of snapshots of the data stream, as many as will fit within the
specified time interval. For the range queries the Prometheus Source runs, the
start time is the previous time the query ran and the end time is now, with the
length of this time interval determined by the _schedule_ property.

For example, the following CR specifies a range query
`go_memstats_alloc_bytes{instance="demo.robustperception.io:9090",job="prometheus"}`
to be run every minute with resolution step of 15 seconds:

```yaml
apiVersion: sources.eventing.knative.dev/v1alpha1
kind: PrometheusSource
metadata:
  name: prometheus-source
spec:
  serverURL: http://demo.robustperception.io:9090
  promQL: 'go_memstats_alloc_bytes{instance="demo.robustperception.io:9090",job="prometheus"}'
  schedule: "* * * * *"
  step: 15s
  sink:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: event-display
```

This will produce a CloudEvent every minute, each covering a period of one
minute between one minute ago and now, inclusive, and containing 5 samples (15
seconds between samples) of go_memstats_alloc_bytes of the job `prometheus` on
the Prometheus instance `demo.robustperception.io:9090`.

## Using the Prometheus Event Source with an off-cluster Prometheus server

1. Set up [Knative Serving, Knative Eventing](../DEVELOPMENT.md)

1. Deploy an event-display sink for the events produced by the Prometheus
   source:

```bash
kubectl apply -f demo/sink.yaml
```

1. Deploy the Prometheus source, configured to communicate with an off-cluster
   Prometheus server:

```bash
kubectl apply -f demo/source.yaml
```

1. Tail the log of the even-display sink to watch the CloudEvents produced by
   the Prometheus source. There may be a brief pause while Knative Serving
   launches the event-display pod.

```bash
kubectl logs -f $(k get pods | grep event-display | awk '{print $1}') -c user-container
```

1. A PrometheusSource CR specifies the URL for the Prometheus server, the PromQL
   query to run, the crontab-formatted schedule for how often to run the PromQL
   query and the sink to send CloudEvents to. demo/source.yaml instructs the
   source to retrieve active alerts every minute:

```yaml
apiVersion: sources.eventing.knative.dev/v1alpha1
kind: PrometheusSource
metadata:
  name: prometheus-source
spec:
  serverURL: http://demo.robustperception.io:9090
  promQL: ALERTS
  schedule: "* * * * *"
  sink:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: event-display
```

Please note that demo.robustperception.io is a publicly accessible Prometheus
server that is outside of Knative project's control.

## Using the Prometheus Event Source with Knative Monitoring

1. Set up [Knative Serving, Knative Eventing](../DEVELOPMENT.md) and
   [Knative Monitoring](https://knative.dev/docs/serving/installing-logging-metrics-traces/)
1. Enable collection of Knative Serving request metrics by setting
   `metrics.request-metrics-backend-destination: prometheus` in the
   config-observability ConfigMap:

```bash
kubectl edit cm -n knative-serving config-observability
```

1. Deploy an event-display sink for the events produced by the Prometheus
   source:

```bash
kubectl apply -f demo/sink.yaml
```

1. Create a CR for the source, configured to communicate with the in-cluster
   Prometheus server deployed as part of Knative Monitoring:

```bash
kubectl apply -f demo/source_knative.yaml
```

1. Tail the log of the even-display sink to watch the CloudEvents produced by
   the Prometheus source. There may be a brief pause while Knative Serving
   launches the event-display pod.

```bash
kubectl logs -f $(k get pods | grep event-display | awk '{print $1}') -c user-container
```

The Prometheus server in Knative Monitoring is fronted by the
prometheus-system-discovery service in the knative-monitoring namespace, which
determines the value of the serverURL property. The PromQL query retrieves the
monotonically increasing number of requests that the event-display service has
handled so far, making this example feed off its own activity:

```yaml
apiVersion: sources.eventing.knative.dev/v1alpha1
kind: PrometheusSource
metadata:
  name: prometheus-source
spec:
  serverURL: http://prometheus-system-discovery.knative-monitoring.svc.cluster.local:9090
  promQL: 'revision_app_request_count{service_name="event-display"}'
  schedule: "* * * * *"
  step: 15s
  sink:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: event-display
```

## Using the Prometheus Event Source with OpenShift Monitoring Stack

The following assumes deployment of the Prometheus Source into the default
project to run under the default service account.

1. Set up [Knative Serving and Knative Eventing](../DEVELOPMENT.md)
1. Create a ConfigMap for the Service signer CA bundle and annotate it
   accordingly:

```bash
oc apply -f demo/openshift-service-serving-signer-cabundle.yaml
oc annotate configmap openshift-service-serving-signer-cabundle service.beta.openshift.io/inject-cabundle=true
```

1. Bind the cluster-monitoring-view ClusterRole to the default service account:

```bash
oc adm policy add-cluster-role-to-user cluster-monitoring-view system:serviceaccount:default:default
```

1. Deploy an event-display sink for the events produced by the Prometheus
   source:

```bash
oc apply -f demo/sink.yaml
```

1. Create a CR for the source, configured to communicate with the in-cluster
   Prometheus server. The authTokenFile field is the conventional location for
   the service account authentication token. The caCertConfigMap is the name of
   the Service signer CA ConfigMap.

```bash
oc apply -f demo/source_openshift.yaml
```

1. Tail the log of the even-display sink to watch the CloudEvents produced by
   the Prometheus source. There may be a brief pause while Knative Serving
   launches the event-display pod.

```bash
oc logs -f $(k get pods | grep event-display | awk '{print $1}') -c user-container
```
