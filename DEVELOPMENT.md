Development
This doc explains how to set up a development environment so you can get started contributing to Knative Eventing. Al l so take a look at the development workflow and the test docs.

Getting started
Setup Knative Serving
Setup Knative Eventing
Create and checkout a repo fork
Once you meet these requirements, you can install a source!

Before submitting a PR, see also CONTRIBUTING.md.

Requirements
You must have the core of Knative running on your cluster.

You must have Knative Eventing running on your cluster.

You must have ko installed.

Check out your fork
The Go tools require that you clone the repository to the src/github.com/knative/eventing-sources directory in your GOPATH.

To check out this repository:

Create your own fork of this repo
Clone it to your machine:
mkdir -p ${GOPATH}/src/github.com/k native
cd ${GOPATH}/src/github.com/k native
git clone git@github.com:${YOUR_GITHUB_USERNAME}/eventing-sources.git
cd eventing-sources
git remote add upstream git@github.com:knative/eventing-sources.git
git remote set-url --push upstream no_push
Adding the upstream remote sets you up nicely for regularly syncing your fork.

Once you reach this point you are ready to do a full build and deploy as follows.

Installing a Source
Once you've set up your development environment, install all sources except gcp  pubsub with:

ko apply -f config/
This command is idempotent, so you can run it at any time to update your deployment.

See contrib/gcp pubsub/samples/README.md for instructions on installing the gcp pubsub source.

You can see things running with:

$ kubectl -n knative-sources get pods
NAME                   READY     STATUS    RESTARTS   AGE
controller-manager-0   1/1       Running   0          2h
You can access the Eventing Manager's logs with:

kubectl -n knative-sources logs $(kubectl -n knative-sources get pods -l control-plane=controller-manager -o name)
Iterating
As you make changes to the code-base:

If you change a package's deps (including adding external dep), then you must run ./hack/update-deps.sh.
If you change a type definition (pkg/apis/), then you must run ./hack/update-codegen.sh. This also runs ./hack/update-deps.sh.
These are both idempotent, and we expect that running these in the master branch to produce no diffs.

To verify that your generated code is correct with the new type definition you can run ./hack/verify-code gen.sh. On OSX you will need GNU diff version 3.7 that you can install from brew with brew install diffutils.

To check that the build and tests passes please see the test documentation or simply run ./test/pre submit - tests.sh.

Once the code gen and dependency information is correct, redeploy using the same to apply command you used Installing a Source.

Or you can clean it up completely and start again.

Tests
Running tests as you make changes to the code-base is pretty simple. See the test docs.

Clean up
You can delete Knative Sources with:

ko delete -f config/
