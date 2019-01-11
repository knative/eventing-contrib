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

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/library.sh

function run_kustomize() {
  run_go_tool sigs.k8s.io/kustomize kustomize "$@"
}

# We've had to make local changes to controller-gen in the past, so always run
# it from vendor rather than upstream.
# TODO(n3wscott): Is this still needed?
function run_controller_gen() {
  go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go "$@"
}

cd ${REPO_ROOT_DIR}

# Ensure we have everything we need under vendor/
${REPO_ROOT_DIR}/hack/update-deps.sh

# Generate manifests e.g. CRD, RBAC etc.
run_controller_gen all

# Generate the default config from kustomize.
run_kustomize build config/default/ > config/default.yaml

run_kustomize build config/awssqs > config/default-awssqs.yaml

# To deploy controller in the configured Kubernetes cluster in ~/.kube/config
# run_kustomize build config/default | ko apply -f /dev/stdin/

# Items in contrib will need the above tools applied separately for each
# directory.

PUBSUB_INPUT_DIR="$(realpath contrib/gcppubsub)"
PUBSUB_OUTPUT_DIR="$(realpath contrib/gcppubsub/config)"
run_controller_gen crd --root-path="${PUBSUB_INPUT_DIR}" --output-dir="${PUBSUB_OUTPUT_DIR}"
# `run_controller_gen all` doesn't support setting the --name flag for rbac,
# which is needed to avoid collisions between the gcppubsub controller and the
# default controller.
run_controller_gen rbac --name=gcppubsub-controller --input-dir="${PUBSUB_INPUT_DIR}/pkg" --output-dir="${PUBSUB_OUTPUT_DIR}"

# Generate the default config that includes gcppubsub.
run_kustomize build contrib/gcppubsub/config/ > contrib/gcppubsub/config/default-gcppubsub.yaml
