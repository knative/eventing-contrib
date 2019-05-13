#!/usr/bin/env bash

# Copyright 2018 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script runs the end-to-end tests against the eventing sources
# built from source.

# If you already have the `KO_DOCKER_REPO` environment variable set and a
# cluster setup and currently selected in your kubeconfig, call the script
# with the `--run-tests` argument and it will use the cluster and run the tests.

# Calling this script without arguments will create a new cluster in
# project $PROJECT_ID, start Knative serving and the eventing system, run
# the tests and delete the cluster.

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/e2e-tests.sh

# Names of the Resources used in the tests.
readonly E2E_TEST_NAMESPACE=e2etest

# Helper functions.

function knative_setup() {
  start_latest_knative_serving || return 1
  start_latest_knative_eventing || return 1

  header "Standing up Knative Eventing Sources"
  ko apply -f config/ || return 1
  wait_until_pods_running knative-sources || fail_test "Eventing Sources did not come up"
}

# Install the latest stable Knative/eventing in the current cluster.
function start_latest_knative_eventing() {
  header "Starting Knative Eventing"
  subheader "Installing Knative Eventing"
  kubectl apply -f ${KNATIVE_EVENTING_RELEASE} || return 1
  wait_until_pods_running knative-eventing || return 1
}


function knative_teardown() {
  ko delete --ignore-not-found=true -f config/
  ko delete --ignore-not-found=true -f ${KNATIVE_EVENTING_RELEASE}

  wait_until_object_does_not_exist namespaces knative-sources
  wait_until_object_does_not_exist namespaces knative-eventing

  wait_until_object_does_not_exist customresourcedefinitions containersources.sources.eventing.knative.dev
  wait_until_object_does_not_exist customresourcedefinitions githubsources.sources.eventing.knative.dev
}

function test_setup() {
  kubectl create namespace $E2E_TEST_NAMESPACE || return 1
  kubectl label namespace $E2E_TEST_NAMESPACE istio-injection=enabled --overwrite || return 1
  # Publish test images
  $(dirname $0)/upload-test-images.sh e2e || fail_test "Error uploading test images"
}

function test_teardown() {
  # Delete the test namespace
  echo "Deleting namespace $E2E_TEST_NAMESPACE"
  kubectl --ignore-not-found=true delete namespace $E2E_TEST_NAMESPACE
  wait_until_object_does_not_exist namespaces $E2E_TEST_NAMESPACE
}

function dump_extra_cluster_state() {
  # Collecting logs from all knative's eventing and eventing-sources pods
  echo "============================================================"
  for namespace in "knative-eventing" "knative-sources" "$E2E_TEST_NAMESPACE"; do
    for pod in $(kubectl get pod -n $namespace | grep Running | awk '{print $1}' ); do
      for container in $(kubectl get pod "${pod}" -n $namespace -ojsonpath='{.spec.containers[*].name}'); do
        echo "Namespace, Pod, Container: ${namespace}, ${pod}, ${container}"
        kubectl logs -n $namespace "${pod}" -c "${container}" || true
        echo "----------------------------------------------------------"
        echo "Namespace, Pod, Container (Previous instance): ${namespace}, ${pod}, ${container}"
        kubectl logs -p -n $namespace "${pod}" -c "${container}" || true
        echo "============================================================"
      done
    done
  done
}

# Script entry point.

initialize $@

go_test_e2e ./test/e2e || fail_test

success
