# Prometheus Event Source for Knative Eventing

The Prometheus Event Source enables Knative Eventing integration with the
[Prometheus Monitoring System](https://prometheus.io/).

At the moment the implementation supports only simple PromQL queries. The
queries run against the Prometheus server every 5 seconds and the results are
sent to the configured sink as CloudEvents.

## Deployment steps

1. Setup [Knative Eventing](../DEVELOPMENT.md)
1. Create a PrometheusSource CRD, specifying the URL for the Prometheus server,
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
       apiVersion: serving.knative.dev/v1
       kind: Service
       name: event-display
   ```
