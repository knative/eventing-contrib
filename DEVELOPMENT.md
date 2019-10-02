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

You must have the core of [Knative Serving](http://github.com/knative/serving)
running on your cluster.

You must have [Knative Eventing](http://github.com/knative/eventing) running on
your cluster.

You must have [ko](https://github.com/google/ko) installed.

### Checkout your fork

The Go tools require that you clone the repository to the
`src/knative.dev/eventing-contrib` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own
   [fork of this repo](https://help.github.com/articles/fork-a-repo/)
1. Clone it to your machine:

```shell
mkdir -p ${GOPATH}/src/knative.dev
cd ${GOPATH}/src/knative.dev
git clone git@github.com:${YOUR_GITHUB_USERNAME}/eventing-contrib.git
cd eventing-contrib
git remote add upstream git@github.com:knative/eventing-contrib.git
git remote set-url --push upstream no_push
```

_Adding the `upstream` remote sets you up nicely for regularly
[syncing your fork](https://help.github.com/articles/syncing-a-fork/)._

Once you reach this point you are ready to do a full build and deploy as
follows.

## Installing Sources

Once you've [setup your development environment](#getting-started), install any
of the sources _Github Source_, _AWS SQS Source_, _Camel Source_, _Kafka Source_
with:

```
ko apply -f <source_name>/config  # e.g. github/config
```

These commands are idempotent, so you can run them at any time to update your
deployment.

If you applied the _GitHub Source_, you can see things running with:

```shell
$ kubectl -n knative-sources get pods
NAME                          READY     STATUS    RESTARTS   AGE
github-controller-manager-0   1/1       Running   0          2h
```

You can access the Github eventing manager's logs with:

```shell
kubectl -n knative-sources logs \
    $(kubectl \
        -n knative-sources \
        get pods \
        -l control-plane=github-controller-manager \
        -o name \
    )
```

_See [camel/source/samples/README.md](./camel/source/samples/README.md),
[kafka/source/README.md](./kafka/source/README.md) for instructions on
installing the Camel Source and Kafka Source._

## Iterating

As you make changes to the code-base:

- **If you change a package's deps** (including adding external dep), then you
  must run [`./hack/update-deps.sh`](./hack/update-deps.sh).
- **If you change a type definition (\<source_name\>/pkg/apis/),** then you must
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
ko delete -f <source_name>/config/
```

<!--
TODO(#15): Add default telemetry.
## Telemetry

See [telemetry documentation](./docs/telemetry.md).
-->
