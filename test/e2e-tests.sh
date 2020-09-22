#!/usr/bin/env bash

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

# Eventing main config path from HEAD.
readonly EVENTING_CONFIG="./config/"
readonly EVENTING_MT_CHANNEL_BROKER_CONFIG="./config/brokers/mt-channel-broker"
readonly EVENTING_IN_MEMORY_CHANNEL_CONFIG="./config/channels/in-memory-channel"

# Vendored eventing test iamges.
readonly VENDOR_EVENTING_TEST_IMAGES="vendor/knative.dev/eventing/test/test_images/"
# HEAD eventing test images.
readonly HEAD_EVENTING_TEST_IMAGES="${GOPATH}/src/knative.dev/eventing/test/test_images/"

# Config tracing config.
readonly CONFIG_TRACING_CONFIG="test/config/config-tracing.yaml"

# Strimzi installation config template used for starting up Kafka clusters.
readonly STRIMZI_INSTALLATION_CONFIG_TEMPLATE="test/config/100-strimzi-cluster-operator-0.19.0.yaml"
# Strimzi installation config.
readonly STRIMZI_INSTALLATION_CONFIG="$(mktemp)"
# Kafka cluster CR config file.
readonly KAFKA_INSTALLATION_CONFIG="test/config/100-kafka-ephemeral-triple-2.5.0.yaml"
# Kafka cluster URL for our installation
readonly KAFKA_CLUSTER_URL="my-cluster-kafka-bootstrap.kafka:9092"
# Kafka channel CRD config template directory.
readonly KAFKA_CRD_CONFIG_TEMPLATE_DIR="kafka/channel/config"
# Kafka channel CRD config template file. It needs to be modified to be the real config file.
readonly KAFKA_CRD_CONFIG_TEMPLATE="400-kafka-config.yaml"
# Real Kafka channel CRD config , generated from the template directory and modified template file.
readonly KAFKA_CRD_CONFIG_DIR="$(mktemp -d)"
# Kafka channel CRD config template directory.
readonly KAFKA_SOURCE_CRD_CONFIG_DIR="kafka/source/config"

function knative_setup() {
  if is_release_branch; then
    echo ">> Install Knative Eventing from ${KNATIVE_EVENTING_RELEASE}"
    kubectl apply -f ${KNATIVE_EVENTING_RELEASE}
  else
    echo ">> Install Knative Eventing from HEAD"
    pushd .
    cd ${GOPATH} && mkdir -p src/knative.dev && cd src/knative.dev
    git clone https://github.com/knative/eventing
    cd eventing
    ko apply -f ${EVENTING_CONFIG}
    # Install MT Channel Based Broker
    ko apply -f ${EVENTING_MT_CHANNEL_BROKER_CONFIG}
    # Install IMC
    ko apply -f ${EVENTING_IN_MEMORY_CHANNEL_CONFIG}
    popd
  fi
  wait_until_pods_running knative-eventing || fail_test "Knative Eventing did not come up"

  # Setup config tracing for tracing tests
  kubectl replace -f $CONFIG_TRACING_CONFIG

  # TODO install head if !is_release_branch
  echo "Installing Knative Monitoring"
  # Hack hack hack. Why is this namespace not created as part of monitoring release.
  # https://github.com/knative/eventing/issues/3469
  kubectl create ns knative-monitoring
  kubectl create namespace istio-system
  kubectl apply --filename "${KNATIVE_MONITORING_RELEASE}" || return 1
  wait_until_pods_running istio-system || fail_test "Knative Monitoring did not come up"
}

function knative_teardown() {
  echo ">> Stopping Knative Eventing"
  if is_release_branch; then
    echo ">> Uninstalling Knative Eventing from ${KNATIVE_EVENTING_RELEASE}"
    kubectl delete -f ${KNATIVE_EVENTING_RELEASE}
  else
    echo ">> Uninstalling Knative Eventing from HEAD"
    kubectl delete --ignore-not-found=true --now --timeout 60s -f ${EVENTING_CONFIG}
  fi
  wait_until_object_does_not_exist namespaces knative-eventing
}

# Add function call to trap
# Parameters: $1 - Function to call
#             $2...$n - Signals for trap
function add_trap() {
  local cmd=$1
  shift
  for trap_signal in $@; do
    local current_trap="$(trap -p $trap_signal | cut -d\' -f2)"
    local new_cmd="($cmd)"
    [[ -n "${current_trap}" ]] && new_cmd="${current_trap};${new_cmd}"
    trap -- "${new_cmd}" $trap_signal
  done
}

function test_setup() {
  kafka_setup || return 1

  install_channel_crds || return 1
  install_sources_crds || return 1

  # Install kail if needed.
  if ! which kail > /dev/null; then
    bash <( curl -sfL https://raw.githubusercontent.com/boz/kail/master/godownloader.sh) -b "$GOPATH/bin"
  fi

  # Capture all logs.
  kail > ${ARTIFACTS}/k8s.log.txt &
  local kail_pid=$!
  # Clean up kail so it doesn't interfere with job shutting down
  add_trap "kill $kail_pid || true" EXIT

  # Publish test images.
  echo ">> Publishing test images from eventing"
  # We vendor test image code from eventing, in order to use ko to resolve them into Docker images, the
  # path has to be a GOPATH.
  sed -i 's@knative.dev/eventing/test/test_images@knative.dev/eventing-contrib/vendor/knative.dev/eventing/test/test_images@g' "${VENDOR_EVENTING_TEST_IMAGES}"*/*.yaml
  $(dirname $0)/upload-test-images.sh ${VENDOR_EVENTING_TEST_IMAGES} e2e || fail_test "Error uploading test images"
  $(dirname $0)/upload-test-images.sh "test/test_images" e2e || fail_test "Error uploading test images"

  # Setup config tracing for tracing tests
  kubectl replace -f $CONFIG_TRACING_CONFIG
}

function test_teardown() {
  kafka_teardown

  uninstall_channel_crds
  uninstall_sources_crds
}

function install_channel_crds() {
  echo "Installing Kafka Channel CRD"
  cp ${KAFKA_CRD_CONFIG_TEMPLATE_DIR}/*yaml ${KAFKA_CRD_CONFIG_DIR}
  sed -i "s/REPLACE_WITH_CLUSTER_URL/${KAFKA_CLUSTER_URL}/" ${KAFKA_CRD_CONFIG_DIR}/${KAFKA_CRD_CONFIG_TEMPLATE}
  ko apply -f ${KAFKA_CRD_CONFIG_DIR} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the Kafka Channel CRD"
}

function install_sources_crds() {
  echo "Installing Kafka Source CRD"
  ko apply -f ${KAFKA_SOURCE_CRD_CONFIG_DIR} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the Kafka Source CRD"
  wait_until_pods_running knative-sources || fail_test "Failed to install the Kafka Source CRD"
}

function uninstall_channel_crds() {
  echo "Uninstalling Kafka Channel CRD"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${KAFKA_CRD_CONFIG_DIR}
}

function uninstall_sources_crds() {
  echo "Uninstalling Kafka Source CRD"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${KAFKA_SOURCE_CRD_CONFIG_DIR}
}

function kafka_setup() {
  echo "Installing Kafka cluster"
  kubectl create namespace kafka || return 1
  sed 's/namespace: .*/namespace: kafka/' ${STRIMZI_INSTALLATION_CONFIG_TEMPLATE} > ${STRIMZI_INSTALLATION_CONFIG}
  kubectl apply -f "${STRIMZI_INSTALLATION_CONFIG}" -n kafka
  kubectl apply -f ${KAFKA_INSTALLATION_CONFIG} -n kafka
  wait_until_pods_running kafka || fail_test "Failed to start up a Kafka cluster"
}

function kafka_teardown() {
  echo "Uninstalling Kafka cluster"
  kubectl delete -f ${KAFKA_INSTALLATION_CONFIG} -n kafka
  kubectl delete -f "${STRIMZI_INSTALLATION_CONFIG}" -n kafka
  kubectl delete namespace kafka
}

initialize $@ --skip-istio-addon

go_test_e2e -timeout=30m -parallel=12 ./test/e2e -channels=messaging.knative.dev/v1alpha1:KafkaChannel,messaging.knative.dev/v1beta1:KafkaChannel  || fail_test

go_test_e2e -timeout=5m -parallel=12 ./test/conformance -channels=messaging.knative.dev/v1beta1:KafkaChannel -sources=sources.knative.dev/v1beta1:KafkaSource || fail_test

# If you wish to use this script just as test setup, *without* teardown, just uncomment this line and comment all go_test_e2e commands
# trap - SIGINT SIGQUIT SIGTSTP EXIT

success
