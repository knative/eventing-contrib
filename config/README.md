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

The GCP PubSub source is not active in the `default.yaml` config. Add it by using the `kustomization.yaml` file in `gcppubsub` (instead of the one in `default`):

```shell
kustomize build config/gcppubsub/ > custom.yaml
ko apply -f custom.yaml
```

or use the prebuilt

```shell
ko apply -f config/default-gcppubsub.yaml
```
