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
apiVersion: sources.knative.dev/v1alpha1
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

- Set up [Knative Serving, Knative Eventing](../DEVELOPMENT.md)

- Deploy an event-display sink for the events produced by the Prometheus source:

```bash
kubectl apply -f demo/sink.yaml
```

- Deploy the Prometheus source, configured to communicate with an off-cluster
  Prometheus server:

```bash
kubectl apply -f demo/source.yaml
```

- Tail the log of the even-display sink to watch the CloudEvents produced by the
  Prometheus source. There may be a brief pause while Knative Serving launches
  the event-display pod.

```bash
kubectl logs -f -l serving.knative.dev/service=event-display -c user-container
```

- A PrometheusSource CR specifies the URL for the Prometheus server, the PromQL
  query to run, the crontab-formatted schedule for how often to run the PromQL
  query and the sink to send CloudEvents to. demo/source.yaml instructs the
  source to retrieve active alerts every minute:

```yaml
apiVersion: sources.knative.dev/v1alpha1
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

**Note:** demo.robustperception.io is a publicly accessible Prometheus server
that is outside of Knative project's control.

## Using the Prometheus Event Source with Knative Monitoring

- Set up [Knative Serving, Knative Eventing](../DEVELOPMENT.md) and
  [Knative Monitoring](https://knative.dev/docs/serving/installing-logging-metrics-traces/)
- Enable collection of Knative Serving request metrics by setting
  `metrics.request-metrics-backend-destination: prometheus` in the
  config-observability ConfigMap:

```bash
kubectl edit cm -n knative-serving config-observability
```

- Deploy an event-display sink for the events produced by the Prometheus source:

```bash
kubectl apply -f demo/sink.yaml
```

- Create a CR for the source, configured to communicate with the in-cluster
  Prometheus server deployed as part of Knative Monitoring:

```bash
kubectl apply -f demo/source_knative.yaml
```

- Tail the log of the even-display sink to watch the CloudEvents produced by the
  Prometheus source. There may be a brief pause while Knative Serving launches
  the event-display pod.

```bash
kubectl logs -f -l serving.knative.dev/service=event-display -c user-container
```

The Prometheus server in Knative Monitoring is fronted by the
prometheus-system-discovery service in the knative-monitoring namespace, which
determines the value of the serverURL property. The PromQL query retrieves the
monotonically increasing number of requests that the event-display service has
handled so far, making this example feed off its own activity:

```yaml
apiVersion: sources.knative.dev/v1alpha1
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

- Set up [Knative Serving and Knative Eventing](../DEVELOPMENT.md)
- Create a ConfigMap for the Service signer CA bundle and annotate it
  accordingly:

```bash
oc apply -f demo/openshift-service-serving-signer-cabundle.yaml
oc annotate configmap openshift-service-serving-signer-cabundle service.beta.openshift.io/inject-cabundle=true
```

- Bind the cluster-monitoring-view ClusterRole to the default service account:

```bash
oc adm policy add-cluster-role-to-user cluster-monitoring-view system:serviceaccount:default:default
```

- Deploy an event-display sink for the events produced by the Prometheus source:

```bash
oc apply -f demo/sink.yaml
```

- Create a CR for the source, configured to communicate with the in-cluster
  Prometheus server. The authTokenFile field is the conventional location for
  the service account authentication token. The caCertConfigMap is the name of
  the Service signer CA ConfigMap.

```bash
oc apply -f demo/source_openshift.yaml
```

- Tail the log of the even-display sink to watch the CloudEvents produced by the
  Prometheus source. There may be a brief pause while Knative Serving launches
  the event-display pod.

```bash
oc logs -f -l serving.knative.dev/service=event-display -c user-container
```

## Using Triggers to filter events from the Prometheus Event Source

This example builds from many of the same building blocks as
[Using the Prometheus Event Source with Knative Monitoring](#user-content-using-the-prometheus-event-source-with-knative-monitoring).

- Set up [Knative Serving, Knative Eventing](../DEVELOPMENT.md) and
  [Knative Monitoring](https://knative.dev/docs/serving/installing-logging-metrics-traces/)
- Enable collection of Knative Serving request metrics by setting
  `metrics.request-metrics-backend-destination: prometheus` in the
  config-observability ConfigMap:

```bash
kubectl edit cm -n knative-serving config-observability
```

- Check that the In-Memory Channel is deployed. There should be the
  imc-controller and imc-dispatcher pods running in the knative-eventing
  namespace:

```bash
$ kubectl get pods -n knative-eventing
NAME                                  READY   STATUS    RESTARTS   AGE
...
imc-controller-7fcd9b75fc-d5nnx       1/1     Running   0          105m
imc-dispatcher-85b9b77759-pcmsr       1/1     Running   0          105m
...
$
```

- Create the default Broker in the default namespace:

```bash
kubectl label namespace default knative-eventing-injection=enabled
```

- Check that the default Broker is running:

```bash
$ kubectl get brokers
NAME      READY   REASON   URL                                               AGE
default   True             http://default-broker.default.svc.cluster.local   115m
$
```

- Deploy two Prometheus Sources with the default Broker as their sink, two
  event-display Knative Services that serve as the ultimate destinations for the
  Prometheus events and two Triggers that subscribe to events from the
  corresponding sources and forward them to the event-display sinks:

```bash
kubectl apply -f demo/broker_trigger.yaml
```

- Check the event types now available from the Broker. The type of events
  generated by the Prometheus Event Source is 'dev.knative.prometheus.promql'.
  The event sources are named according to the 'namespace/source-name'
  convention:

```bash
[syedriko@localhost prometheus]$ k get eventtypes
NAME                                  TYPE                            SOURCE                    SCHEMA   BROKER    DESCRIPTION   READY   REASON
dev.knative.prometheus.promql-j7ltk   dev.knative.prometheus.promql   default/request-count-2            default                 True
dev.knative.prometheus.promql-t4rc9   dev.knative.prometheus.promql   default/request-count-1            default                 True
```

- The Triggers filter the events based on their type and source:

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Trigger
metadata:
  name: request-count-1
spec:
  broker: default
  filter:
    sourceAndType:
      type: dev.knative.prometheus.promql
      source: default/request-count-1
  subscriber:
    ref:
      apiVersion: serving.knative.dev/v1
      kind: Service
      name: event-display-1
```

- Tail the logs of the even-display sinks to watch the CloudEvents produced by
  the Prometheus sources and note that event-display-1 only receives events from
  source request-count-1 and event-display-2 only receives events from source
  request-count-2:

```bash
$ kubectl logs -f -l serving.knative.dev/service=event-display-1 -c user-container
...
cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 0.3
  type: dev.knative.prometheus.promql
  source: default/request-count-1
...
```

```bash
$ kubectl logs -f -l serving.knative.dev/service=event-display-2 -c user-container
...
cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 0.3
  type: dev.knative.prometheus.promql
  source: default/request-count-2
...
```

There may be a brief pause while Knative Serving launches the event-display
pods.
