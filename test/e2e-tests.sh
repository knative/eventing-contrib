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

# Eventing main config path from HEAD.
readonly EVENTING_CONFIG="${GOPATH}/src/knative.dev/eventing/config/"

# Vendored eventing test iamges.
readonly VENDOR_EVENTING_TEST_IMAGES="vendor/knative.dev/eventing/test/test_images/"
# HEAD eventing test images.
readonly HEAD_EVENTING_TEST_IMAGES="${GOPATH}/src/knative.dev/eventing/test/test_images/"

# NATS Streaming installation config.
readonly NATSS_INSTALLATION_CONFIG="natss/config/broker/natss.yaml"
# NATSS channel CRD config directory.
readonly NATSS_CRD_CONFIG_DIR="natss/config"

# Strimzi installation config template used for starting up Kafka clusters.
readonly STRIMZI_INSTALLATION_CONFIG_TEMPLATE="test/config/100-strimzi-cluster-operator-0.16.2.yaml"
# Strimzi installation config.
readonly STRIMZI_INSTALLATION_CONFIG="$(mktemp)"
# Kafka cluster CR config file.
readonly KAFKA_INSTALLATION_CONFIG="test/config/100-kafka-ephemeral-triple-2.4.0.yaml"
readonly KAFKA_TOPIC_INSTALLATION_CONFIG="test/config/100-kafka-topic.yaml"
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

# CamelK installation
readonly CAMELK_INSTALLATION_CONFIG="test/config/100-camel-k-1.0.0-RC2.yaml"
# Camel source CRD config template directory
readonly CAMEL_SOURCE_CRD_CONFIG_DIR="camel/source/config"


function knative_setup() {
  if is_release_branch; then
    echo ">> Install Knative Eventing from ${KNATIVE_EVENTING_RELEASE}"
    kubectl apply -f ${KNATIVE_EVENTING_RELEASE}
  else
    echo ">> Install Knative Eventing from HEAD"
    pushd .
    cd ${GOPATH} && mkdir -p src/knative.dev && cd src/knative.dev
    git clone https://github.com/knative/eventing
    popd
    ko apply -f ${EVENTING_CONFIG}
  fi
  wait_until_pods_running knative-eventing || fail_test "Knative Eventing did not come up"

  # TODO install head if !is_release_branch
  echo "Installing Knative Monitoring"
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
    ko delete --ignore-not-found=true --now --timeout 60s -f ${EVENTING_CONFIG}
  fi
  wait_until_object_does_not_exist namespaces knative-eventing
}

function test_setup() {
  natss_setup || return 1
  kafka_setup || return 1
  camel_setup || return 1

  install_channel_crds || return 1
  install_sources_crds || return 1

  # Publish test images.
  echo ">> Publishing test images from vendor"
  # We vendor test image code from eventing, in order to use ko to resolve them into Docker images, the
  # path has to be a GOPATH.
  sed -i 's@knative.dev/eventing/test/test_images@knative.dev/eventing-contrib/vendor/knative.dev/eventing/test/test_images@g' "${VENDOR_EVENTING_TEST_IMAGES}"/*/*.yaml
  $(dirname $0)/upload-test-images.sh ${VENDOR_EVENTING_TEST_IMAGES} e2e || fail_test "Error uploading test images"
  $(dirname $0)/upload-test-images.sh "test/test_images" e2e || fail_test "Error uploading test images"
}

function test_teardown() {
  natss_teardown
  kafka_teardown
  camel_teardown

  uninstall_channel_crds
  uninstall_sources_crds
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

function install_sources_crds() {
  echo "Installing Kafka Source CRD"
  ko apply -f ${KAFKA_SOURCE_CRD_CONFIG_DIR} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the Kafka Source CRD"
  wait_until_pods_running knative-sources || fail_test "Failed to install the Kafka Source CRD"

  echo "Installing Camel Source CRD"
  ko apply -f ${CAMEL_SOURCE_CRD_CONFIG_DIR} || return 1
  wait_until_pods_running knative-sources || fail_test "Failed to install the Camel Source CRD"
}

function uninstall_channel_crds() {
  echo "Uninstalling NATSS Channel CRD"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${NATSS_CRD_CONFIG_DIR}

  echo "Uninstalling Kafka Channel CRD"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${KAFKA_CRD_CONFIG_DIR}
}

function uninstall_sources_crds() {
  echo "Uninstalling Kafka Source CRD"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${KAFKA_SOURCE_CRD_CONFIG_DIR}
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
  kubectl apply -f "${STRIMZI_INSTALLATION_CONFIG}" -n kafka
  kubectl apply -f ${KAFKA_INSTALLATION_CONFIG} -n kafka
  # kubectl apply -f ${KAFKA_TOPIC_INSTALLATION_CONFIG} -n kafka
  wait_until_pods_running kafka || fail_test "Failed to start up a Kafka cluster"
}

function kafka_teardown() {
  echo "Uninstalling Kafka cluster"
  kubectl delete -f ${KAFKA_TOPIC_INSTALLATION_CONFIG} -n kafka
  kubectl delete -f ${KAFKA_INSTALLATION_CONFIG} -n kafka
  kubectl delete -f "${STRIMZI_INSTALLATION_CONFIG}" -n kafka
  kubectl delete namespace kafka
}

function camel_setup() {
  echo "Installing CamelK"
  kubectl create namespace camelk || return 1
  kubectl apply -f "${CAMELK_INSTALLATION_CONFIG}" -n camelk
}

function camel_teardown() {
  echo "Uninstalling CamelK"
  kubectl delete -f "${CAMELK_INSTALLATION_CONFIG}" -n camelk
  kubectl delete namespace camelk
}

initialize $@ --skip-istio-addon

# TODO: Figure out why kafka channels do not like parallel=12 (which it was)
# https://github.com/knative/eventing-contrib/issues/917
go_test_e2e -timeout=20m -parallel=1 ./test/e2e -channels=messaging.knative.dev/v1alpha1:NatssChannel,messaging.knative.dev/v1alpha1:KafkaChannel  || fail_test

# If you wish to use this script just as test setup, *without* teardown, just uncomment this line and comment all go_test_e2e commands
# trap - SIGINT SIGQUIT SIGTSTP EXIT

success
