# Prometheus Event Source for Knative Eventing

The Prometheus Event Source enables Knative Eventing integration with the
[Prometheus Monitoring System](https://prometheus.io/).

At the moment the implementation supports only simple PromQL queries. The
queries run against the Prometheus server every 5 seconds and the results are
sent to the configured sink as CloudEvents.

## Deployment steps

### Kubernetes

1. Setup [Knative Eventing](../DEVELOPMENT.md)
1. Create a PrometheusSource CR, specifying the URL for the Prometheus server,
   the PromQL query to run, a crontab-formatted schedule for how often to run
   the PromQL query and the sink to send CloudEvents to. The following CR
   instructs the source to retrieve active alerts every 5 minutes:

   ```yaml
   apiVersion: sources.eventing.knative.dev/v1alpha1
   kind: PrometheusSource
   metadata:
     name: prometheus-source
   spec:
     serverURL: http://demo.robustperception.io:9090
     promQL: ALERTS
     schedule: "*/5 * * * *"
     sink:
       ref:
         apiVersion: serving.knative.dev/v1
         kind: Service
         name: event-display
   ```

### Instant and Range Queries

The Prometheus Source supports two kinds of PromQL queries - instant and range.
The _promQL_ property is the basis of a range query if the _step_ property is
specified and an instant query otherwise. An instant query returns a snapshot of
the corresponding data stream at the moment when the query executes on the
server. A range query specifies a time interval and a resolution step and
returns a series of snapshots of the data stream, as many as will fit within the
specified time interval with the given resolution step. For the range queries
the Prometheus Source runs, the start time is the previous time the query ran
and the end time is now, with the length of this time interval determined by the
_schedule_ property.

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

### OpenShift Monitoring Stack

The following assumes deployment of the Prometheus Source into the default
project to run under the default service account.

1. Setup [Knative Eventing](../DEVELOPMENT.md)
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

1. Create a CR for the Source, configured to communicate to the in-cluster
   Prometheus server. The authTokenFile field is the conventional location for
   the service account authentication token. The caCertConfigMap is the name of
   the Service signer CA ConfigMap.

```bash
oc apply -f demo/source_openshift.yaml
```
