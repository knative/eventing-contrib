# Development

This doc explains how to setup a development environment so you can get started
[contributing](./CONTRIBUTING.md) to Knative Eventing. Also take a look at
[the development workflow](./CONTRIBUTING.md#workflow) and
[the test docs](./test/README.md).

## Getting started

1. Setup [Knative Serving](http://github.com/knative/serving)
1. Setup [Knative Eventing](http://github.com/knative/eventing)
1. [Create and checkout a repo fork](#checkout-your-fork)

Once you meet these requirements, you can
[install sources](#installing-sources)!

Before submitting a PR, see also [CONTRIBUTING.md](./CONTRIBUTING.md).

### Requirements

You must have the core of [Knative](http://github.com/knative/serving) running
on your cluster.

You must have [Knative Eventing](http://github.com/knative/serving) running on
your cluster.

You must have
[ko](https://github.com/google/go-containerregistry/blob/master/cmd/ko/README.md)
installed.

### Checkout your fork

The Go tools require that you clone the repository to the
`src/github.com/knative/eventing-sources` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own
   [fork of this repo](https://help.github.com/articles/fork-a-repo/)
1. Clone it to your machine:

```shell
mkdir -p ${GOPATH}/src/github.com/knative
cd ${GOPATH}/src/github.com/knative
git clone git@github.com:${YOUR_GITHUB_USERNAME}/eventing-sources.git
cd eventing-sources
git remote add upstream git@github.com:knative/eventing-sources.git
git remote set-url --push upstream no_push
```

_Adding the `upstream` remote sets you up nicely for regularly
[syncing your fork](https://help.github.com/articles/syncing-a-fork/)._

Once you reach this point you are ready to do a full build and deploy as
follows.

## Installing Sources

Once you've [setup your development environment](#getting-started), install
sources _Github Source_ and _AWS SQS Source_ with:

```shell
ko apply -f config/
```

or install a single source _Camel Source_, _Gcppubsub Source_, _Kafka Source_
with

```
ko apply -f contrib/<source_name>/
```

These commands are idempotent, so you can run them at any time to update your
deployment.

You can see things running with:

```shell
$ kubectl -n knative-sources get pods
NAME                   READY     STATUS    RESTARTS   AGE
controller-manager-0   1/1       Running   0          2h
```

You can access the Eventing Manager's logs with:

```shell
kubectl -n knative-sources logs $(kubectl -n knative-sources get pods -l control-plane=controller-manager -o name)
```

_See
[contrib/gcppubsub/samples/README.md](./contrib/gcppubsub/samples/README.md),
[contrib/camel/samples/README.md](./contrib/camel/samples/README.md),
[contrib/kafka/samples/README.md](./contrib/kafka/samples/README.md) for
instructions on installing the Gcppubsub Source, Camel Source and Kafka Source._

## Iterating

As you make changes to the code-base:

- **If you change a package's deps** (including adding external dep), then you
  must run [`./hack/update-deps.sh`](./hack/update-deps.sh).
- **If you change a type definition ([pkg/apis/](./pkg/apis/.)),** then you must
  run [`./hack/update-codegen.sh`](./hack/update-codegen.sh). _This also runs
  [`./hack/update-deps.sh`](./hack/update-deps.sh)._

These are both idempotent, and we expect that running these in the `master`
branch to produce no diffs.

To verify that your generated code is correct with the new type definition you
can run [`./hack/verify-codegen.sh`](./hack/verify-codegen.sh). On OSX you will
need GNU `diff` version 3.7 that you can install from `brew` with
`brew install diffutils`.

To check that the build and tests passes please see the test
[documentation](#tests) or simply run
[`./test/presubmit-tests.sh`](./test/presubmit-tests.sh).

Once the codegen and dependency information is correct, redeploy using the same
`ko apply` command you used [Installing a Source](#installing-a-source).

Or you can [clean it up completely](#clean-up) and start again.

## Tests

Running tests as you make changes to the code-base is pretty simple. See
[the test docs](./test/README.md).

## Clean up

You can delete `Knative Sources` with:

```shell
ko delete -f config/
```

<!--
TODO(#15): Add default telemetry.
## Telemetry

See [telemetry documentation](./docs/telemetry.md).
-->
