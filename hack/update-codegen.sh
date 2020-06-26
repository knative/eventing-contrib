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

set -o errexit
set -o nounset
set -o pipefail

export GO111MODULE=on
# If we run with -mod=vendor here, then generate-groups.sh looks for vendor files in the wrong place.
export GOFLAGS=-mod=

if [ -z "${GOPATH:-}" ]; then
  export GOPATH=$(go env GOPATH)
fi

source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/library.sh

CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${REPO_ROOT_DIR}; ls -d -1 $(dirname $0)/../vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}

KNATIVE_CODEGEN_PKG=${KNATIVE_CODEGEN_PKG:-$(cd ${REPO_ROOT_DIR}; ls -d -1 $(dirname $0)/../vendor/knative.dev/pkg 2>/dev/null || echo ../pkg)}

chmod +x ${CODEGEN_PKG}/generate-groups.sh
chmod +x ${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh

(
  # External Camel API
  OUTPUT_PKG="knative.dev/eventing-contrib/camel/source/pkg/camel-k/injection" \
  VERSIONED_CLIENTSET_PKG="github.com/apache/camel-k/pkg/client/clientset/versioned" \
  EXTERNAL_INFORMER_PKG="github.com/apache/camel-k/pkg/client/informers/externalversions" \
    ${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh "injection" \
      "knative.dev/eventing-contrib/camel/source/pkg/client/camel" "github.com/apache/camel-k/pkg/apis" \
      "camel:v1" \
      --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate.go.txt
)

# Just Sources
API_DIRS_SOURCES=(camel/source/pkg awssqs/pkg couchdb/source/pkg prometheus/pkg)

for DIR in "${API_DIRS_SOURCES[@]}"; do
  # generate the code with:
  # --output-base    because this script should also be able to run inside the vendor dir of
  #                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
  #                  instead of the $GOPATH directly. For normal projects this can be dropped.
  ${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
    "knative.dev/eventing-contrib/${DIR}/client" "knative.dev/eventing-contrib/${DIR}/apis" \
    "sources:v1alpha1" \
    --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate.go.txt

  # Knative Injection
  ${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh "injection" \
    "knative.dev/eventing-contrib/${DIR}/client" "knative.dev/eventing-contrib/${DIR}/apis" \
    "sources:v1alpha1" \
    --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate.go.txt
done

# Apache Kafka Sources and Bindings
API_DIRS_SOURCES_AND_BINDINGS=(kafka/source/pkg )

for DIR in "${API_DIRS_SOURCES_AND_BINDINGS[@]}"; do
  # generate the code with:
  # --output-base    because this script should also be able to run inside the vendor dir of
  #                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
  #                  instead of the $GOPATH directly. For normal projects this can be dropped.
  ${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
    "knative.dev/eventing-contrib/${DIR}/client" "knative.dev/eventing-contrib/${DIR}/apis" \
    "sources:v1alpha1 sources:v1beta1 bindings:v1alpha1 bindings:v1beta1" \
    --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate.go.txt

  # Knative Injection
  ${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh "injection" \
    "knative.dev/eventing-contrib/${DIR}/client" "knative.dev/eventing-contrib/${DIR}/apis" \
    "sources:v1alpha1 sources:v1beta1 bindings:v1alpha1 bindings:v1beta1" \
    --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate.go.txt
done

# Sources and Bindings
API_DIRS_SOURCES_AND_BINDINGS=(github/pkg gitlab/pkg )

for DIR in "${API_DIRS_SOURCES_AND_BINDINGS[@]}"; do
  # generate the code with:
  # --output-base    because this script should also be able to run inside the vendor dir of
  #                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
  #                  instead of the $GOPATH directly. For normal projects this can be dropped.
  ${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
    "knative.dev/eventing-contrib/${DIR}/client" "knative.dev/eventing-contrib/${DIR}/apis" \
    "sources:v1alpha1 bindings:v1alpha1" \
    --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate.go.txt

  # Knative Injection
  ${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh "injection" \
    "knative.dev/eventing-contrib/${DIR}/client" "knative.dev/eventing-contrib/${DIR}/apis" \
    "sources:v1alpha1 bindings:v1alpha1" \
    --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate.go.txt
done

# Apache Kafka Channels
API_DIRS_CHANNELS=(kafka/channel/pkg)

for DIR in "${API_DIRS_CHANNELS[@]}"; do
  # generate the code with:
  # --output-base    because this script should also be able to run inside the vendor dir of
  #                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
  #                  instead of the $GOPATH directly. For normal projects this can be dropped.
  ${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
    "knative.dev/eventing-contrib/${DIR}/client" "knative.dev/eventing-contrib/${DIR}/apis" \
    "messaging:v1beta1 messaging:v1alpha1" \
    --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate.go.txt

  # Knative Injection
  ${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh "injection" \
    "knative.dev/eventing-contrib/${DIR}/client" "knative.dev/eventing-contrib/${DIR}/apis" \
    "messaging:v1beta1 messaging:v1alpha1" \
    --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate.go.txt
done

# Natss Channels
API_DIRS_CHANNELS=(natss/pkg)

for DIR in "${API_DIRS_CHANNELS[@]}"; do
  # generate the code with:
  # --output-base    because this script should also be able to run inside the vendor dir of
  #                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
  #                  instead of the $GOPATH directly. For normal projects this can be dropped.
  ${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
    "knative.dev/eventing-contrib/${DIR}/client" "knative.dev/eventing-contrib/${DIR}/apis" \
    "messaging:v1alpha1" \
    --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate.go.txt

  # Knative Injection
  ${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh "injection" \
    "knative.dev/eventing-contrib/${DIR}/client" "knative.dev/eventing-contrib/${DIR}/apis" \
    "messaging:v1alpha1" \
    --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate.go.txt
done

# Depends on generate-groups.sh to install bin/deepcopy-gen
${GOPATH}/bin/deepcopy-gen \
  -O zz_generated.deepcopy \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate.go.txt \
  -i knative.dev/eventing-contrib/prometheus/pkg/apis \
  -i knative.dev/eventing-contrib/awssqs/pkg/apis \
  -i knative.dev/eventing-contrib/couchdb/source/pkg/apis \
  -i knative.dev/eventing-contrib/camel/source/pkg/apis \
  -i knative.dev/eventing-contrib/github/pkg/apis \
  -i knative.dev/eventing-contrib/gitlab/pkg/apis

# Make sure our dependencies are up-to-date
${REPO_ROOT_DIR}/hack/update-deps.sh
