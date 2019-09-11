#!/bin/bash

# Copyright 2019 The Knative Authors
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

# This script runs the end-to-end tests against eventing-contrib built
# from source.

# If you already have the *_OVERRIDE environment variables set, call
# this script with the --run-tests arguments and it will use the cluster
# and run the tests.

# Calling this script without arguments will create a new cluster in
# project $PROJECT_ID, start Knative eventing system, install resources
# in eventing-contrib, run the tests and delete the cluster.

source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/e2e-tests.sh

# If gcloud is not available make it a no-op, not an error.
which gcloud &> /dev/null || gcloud() { echo "[ignore-gcloud $*]" 1>&2; }
# Use GNU tools on macOS. Requires the 'grep' and 'gnu-sed' Homebrew formulae.
if [ "$(uname)" == "Darwin" ]; then
  sed=gsed
  grep=ggrep
fi

# Eventing main config.
readonly EVENTING_CONFIG="config/"

# NATS Streaming installation config.
readonly NATSS_INSTALLATION_CONFIG="natss/config/broker/natss.yaml"
# NATSS channel CRD config directory.
readonly NATSS_CRD_CONFIG_DIR="natss/config"

# Strimzi installation config template used for starting up Kafka clusters.
readonly STRIMZI_VERSION="0.11.4"
readonly STRIMZI_INSTALLATION_CONFIG_TEMPLATE="test/config/100-strimzi-cluster-operator-${STRIMZI_VERSION}.yaml"
# Strimzi installation config.
readonly STRIMZI_INSTALLATION_CONFIG="$(mktemp)"
# Kafka cluster CR config file.
readonly KAFKA_INSTALLATION_CONFIG="test/config/100-kafka-persistent-single-2.1.0.yaml"
# Kafka cluster URL for our installation
readonly KAFKA_CLUSTER_URL="my-cluster-kafka-bootstrap.kafka:9092"
# Kafka channel CRD config template directory.
readonly KAFKA_CRD_CONFIG_TEMPLATE_DIR="kafka/channel/config"
# Kafka channel CRD config template file. It needs to be modified to be the real config file.
readonly KAFKA_CRD_CONFIG_TEMPLATE="400-kafka-config.yaml"
# Real Kafka channel CRD config , generated from the template directory and modified template file.
readonly KAFKA_CRD_CONFIG_DIR="$(mktemp -d)"

function knative_setup() {
  if is_release_branch; then
    echo ">> Install Knative Eventing from ${KNATIVE_EVENTING_RELEASE}"
    kubectl apply -f ${KNATIVE_EVENTING_RELEASE}
  else
    echo ">> Install Knative Eventing from HEAD"
    pushd .
    cd ${GOPATH} && mkdir -p src/knative.dev && cd src/knative.dev
    git clone https://github.com/knative/eventing
    cd ${GOPATH}/src/knative.dev/eventing
    ko apply -f ${EVENTING_CONFIG}
    popd
  fi
  wait_until_pods_running knative-eventing || fail_test "Knative Eventing did not come up"
}

function knative_teardown() {
  echo ">> Stopping Knative Eventing"
  echo "Uninstalling Knative Eventing"
  pushd .
  cd ${GOPATH}/src/knative.dev/eventing
  ko delete --ignore-not-found=true --now --timeout 60s -f ${EVENTING_CONFIG}
  popd
  wait_until_object_does_not_exist namespaces knative-eventing
}

function test_setup() {
  natss_setup || return 1
  kafka_setup || return 1

  install_channel_crds || return 1

  # Publish test images.
  echo ">> Publishing test images"
  pushd .
  cd ${GOPATH}/src/knative.dev/eventing
  ./test/upload-test-images.sh e2e || fail_test "Error uploading test images"
  popd
}

function test_teardown() {
  natss_teardown
  kafka_teardown

  uninstall_channel_crds
}

function install_channel_crds() {
  echo "Installing NATSS Channel CRD"
  ko apply -f ${NATSS_CRD_CONFIG_DIR} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the NATSS Channel CRD"

  echo "Installing Kafka Channel CRD"
  cp ${KAFKA_CRD_CONFIG_TEMPLATE_DIR}/*yaml ${KAFKA_CRD_CONFIG_DIR}
  sed -i "s/REPLACE_WITH_CLUSTER_URL/${KAFKA_CLUSTER_URL}/" ${KAFKA_CRD_CONFIG_DIR}/${KAFKA_CRD_CONFIG_TEMPLATE}
  ko apply -f ${KAFKA_CRD_CONFIG_DIR} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the Kafka Channel CRD"
}

function uninstall_channel_crds() {
  echo "Uninstalling NATSS Channel CRD"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${NATSS_CRD_CONFIG_DIR}

  echo "Uninstalling Kafka Channel CRD"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${KAFKA_CRD_CONFIG_DIR}
}

# Create resources required for NATSS provisioner setup
function natss_setup() {
  echo "Installing NATS Streaming"
  kubectl create namespace natss || return 1
  kubectl apply -n natss -f ${NATSS_INSTALLATION_CONFIG} || return 1
  wait_until_pods_running natss || fail_test "Failed to start up a NATSS cluster"
}
# Delete resources used for NATSS provisioner setup
function natss_teardown() {
  echo "Uninstalling NATS Streaming"
  kubectl delete -f ${NATSS_INSTALLATION_CONFIG}
  kubectl delete namespace natss
}

function kafka_setup() {
  echo "Installing Kafka cluster"
  kubectl create namespace kafka || return 1
  sed 's/namespace: .*/namespace: kafka/' ${STRIMZI_INSTALLATION_CONFIG_TEMPLATE} > ${STRIMZI_INSTALLATION_CONFIG}
  kubectl apply -f ${STRIMZI_INSTALLATION_CONFIG} -n kafka
  kubectl apply -f ${KAFKA_INSTALLATION_CONFIG} -n kafka
  wait_until_pods_running kafka || fail_test "Failed to start up a Kafka cluster"
}

function kafka_teardown() {
  echo "Uninstalling Kafka cluster"
  kubectl delete -f ${STRIMZI_INSTALLATION_CONFIG} -n kafka
  kubectl delete -f ${KAFKA_INSTALLATION_CONFIG} -n kafka
  kubectl delete namespace kafka
}

initialize $@ --skip-istio-addon

go_test_e2e -timeout=20m -parallel=12 ./test/e2e -channels=NatssChannel || fail_test

success
