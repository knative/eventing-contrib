# Apache CouchDB source for Knative

The Apache CouchDB Event source enables Knative Eventing integration with
[Apache CouchDB](http://couchdb.apache.org/).

## Deployment steps

1. Setup [Knative Eventing](../DEVELOPMENT.md)
1. Create a secret containing the data needed to access your CouchDB service.
   For example:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: couchdb-binding
   stringData:
     # The URL pointing to the CouchDB http server
     url: "https://username:password/..."
   ```

1. Create the `CouchDbSource` custom objects, by configuring the required
   `credentials` and `database` values on the CR file of your source. Below is
   an example:

   ```yaml
   apiVersion: sources.knative.dev/v1alpha1
   kind: CouchDbSource
   metadata:
     name: couchdb-photographer
   spec:
     # reference to a secret containing the CouchDB credentials
     feed: continuous # default value. For polling every 2 seconds, use "normal"
     credentials:
       name: couchdb-binding
     database: photographers
     sink:
       ref:
         apiVersion: serving.knative.dev/v1alpha1
         kind: Service
         name: event-display
   ```
