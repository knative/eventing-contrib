# Installing

### kustomize

This project uses [kustomize](https://github.com/kubernetes-sigs/kustomize) to
build a custom version of the installation into kubernetes.

To use this:

```shell
kustomize build config/default/ > custom.yaml
```

Then pass this through `ko` to deploy to kubernetes:

```
ko apply -f custom.yaml
```

### Default

The default config has been added for convenience, `ko apply` this to install
the default:

```shell
ko apply -f config/default.yaml
```

### GCP PubSub

The GCP PubSub source is not active in the `default.yaml` config. Add it by
using the `kustomization.yaml` file in `gcppubsub` (instead of the one in
`default`):

```shell
kustomize build config/gcppubsub/ > custom.yaml
ko apply -f custom.yaml
```

or

```shell
kustomize build config/gcppubsub/ | ko apply -f /dev/stdin
```

or use the prebuilt `default-gcppubsub.yaml`.

```shell
ko apply -f config/default-gcppubsub.yaml
```

#### GCP PubSub - Credentials

In addition to [installing the GCP PubSub Controller](#gcp-pubsub), you will
also need to setup the secrets storing GCP credentials.

Create GCP
[Service Account(s)](https://console.cloud.google.com/iam-admin/serviceaccounts/project).
You can either create one or two different Service Accounts. If you create only
one, then use the instructions for the `Source`'s Service Account (which has a
superset of the Receive Adapter's permissions) and provide the same key in both
secrets.

- The `Source`'s Service Account.

  1. Determine the Service Account to use, or create a new one.
  1. Give that Service Account the 'Pub/Sub Editor' role on your GCP project.
  1. Download a new JSON private key for that Service Account.
  1. Create a secret for the downloaded key:

     ```shell
     kubectl -n knative-sources create secret generic gcppubsub-source-key --from-file=key.json=PATH_TO_KEY_FILE.json
     ```

     - Note that you can change the secret's name and the secret's key, but will
       need to modify `default-gcppubsub.yaml` with the updated values (they are
       environment variables on the `StatefulSet`).

- The Receive Adapter's Service Account.

  1. Determine the Service Account to use, or create a new one.
  1. Give that Service Account the 'Pub/Sub Subscriber' role on your GCP
     project.
  1. Download a new JSON private key for that Service Account.
  1. Create a secret for the downloaded key in the namespace that the `Source`
     will be created in:

     ```shell
     kubectl -n default create secret generic google-cloud-key --from-file=key.json=PATH_TO_KEY_FILE.json
     ```

     - Note that you can change the secret's name and the secret's key.
