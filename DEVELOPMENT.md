# Development

This doc explains how to setup a development environment so you can get started
[contributing](./CONTRIBUTING.md) to Knative Eventing. Also take a look at [the
development workflow](./CONTRIBUTING.md#workflow) and [the test
docs](./test/README.md).

## Getting started

1. Setup [Knative Serving](http://github.com/knative/serving)
1. Setup [Knative Eventing](http://github.com/knative/eventing)
1. [Create and checkout a repo fork](#checkout-your-fork)

Once you meet these requirements, you can [install a source](#installing-a-source)!

Before submitting a PR, see also [CONTRIBUTING.md](./CONTRIBUTING.md).

### Requirements

You must have the core of [Knative](http://github.com/knative/serving) running
on your cluster.

You must have [Knative Eventing](http://github.com/knative/serving) running on
your cluster.

You must have
[ko](https://github.com/google/go-containerregistry/blob/master/cmd/ko/README.md
) installed.

### Checkout your fork

The Go tools require that you clone the repository to the
`src/github.com/knative/eventing-sources` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own [fork of this repo](https://help.github.com/articles/fork-a-repo/)
2. Clone it to your machine:
  ```shell
  mkdir -p ${GOPATH}/src/github.com/knative
  cd ${GOPATH}/src/github.com/knative
  git clone git@github.com:${YOUR_GITHUB_USERNAME}/eventing-sources.git
  cd eventing
  git remote add upstream git@github.com:knative/eventing-sources.git
  git remote set-url --push upstream no_push
  ```

_Adding the `upstream` remote sets you up nicely for regularly [syncing your
fork](https://help.github.com/articles/syncing-a-fork/)._

Once you reach this point you are ready to do a full build and deploy as follows.

## Installing a Source

Once you've [setup your development environment](#getting-started), install a
source with:

<!-- TODO(n3wscott): Update to show how to install a single source. -->

```shell
ko apply -f config/
```

You can see things running with:

```shell
$ kubectl -n knative-sources get pods
NAME                       READY     STATUS    RESTARTS   AGE
manager-59f7969778-4dt7l   1/1       Running   0          2h
```

You can access the Eventing Manager's logs with:

```shell
kubectl -n knative-source logs $(kubectl -n knative-souce get pods -l control-plane=controller-manager -o name)
```

## Iterating

As you make changes to the code-base, there are two special cases to be aware of:

- **If you change a type definition ([pkg/apis/](./pkg/apis/.)),** then you must run
  [`./hack/update-codegen.sh`](./hack/update-codegen.sh).
- **If you change a package's deps** (including adding external dep), then you must run
  [`./hack/update-deps.sh`](./hack/update-deps.sh).


These are both idempotent, and we expect that running these at `HEAD` to have
no diffs.

Once the codegen and dependency information is correct, redeploying the
controller is simply:

```shell
ko apply -f config/controller.yaml
```

Or you can [clean it up completely](#clean-up) and start again.

## Tests

Running tests as you make changes to the code-base is pretty simple. See [the
test docs](./test/README.md).

## Clean up

You can delete `Knative Sources` with:

```shell
ko delete -f config/
```

## Telemetry

See [telemetry documentation](./docs/telemetry.md).
