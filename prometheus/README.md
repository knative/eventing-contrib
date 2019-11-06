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
   the PromQL query to run and the sink to send CloudEvents to:

   ```yaml
   apiVersion: sources.eventing.knative.dev/v1alpha1
   kind: PrometheusSource
   metadata:
     name: prometheus-source
   spec:
     serverURL: http://demo.robustperception.io:9090
     promQL: ALERTS
     sink:
       ref:
         apiVersion: serving.knative.dev/v1
         kind: Service
         name: event-display
   ```

### OpenShift Monitoring Stack

The following assumes deployment of the Prometheus Source into the default project to run
under the default service account.

1. Setup [Knative Eventing](../DEVELOPMENT.md)
1. Create a ConfigMap for the Service signer CA bundle and annotate it
   accordingly:

``` bash
oc apply -f demo/openshift-service-serving-signer-cabundle.yaml
oc annotate configmap openshift-service-serving-signer-cabundle service.beta.openshift.io/inject-cabundle=true
```

1. Bind the cluster-monitoring-view ClusterRole to the default service account:

``` bash
oc adm policy add-cluster-role-to-user cluster-monitoring-view system:serviceaccount:default:default
```

1. Deploy an event-display sink for the events produced by the Prometheus source:

``` bash
oc apply -f demo/sink.yaml
```

1. Create a CR for the Source, configured to communicate to the in-cluster
   Prometheus server. The authTokenFile field is the conventional location
   for the service account authentication token. The caCertConfigMap is the
   name of the Service signer CA ConfigMap.

``` bash
oc apply -f demo/source_openshift.yaml
```
