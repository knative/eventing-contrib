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

cd ${REPO_ROOT_DIR}

# Ensure we have everything we need under vendor/
${REPO_ROOT_DIR}/hack/update-deps.sh

# Generate manifests e.g. CRD, RBAC etc.
go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all

# Generate the default config from kustomize.
kustomize build config/default/ > config/default.yaml

# To deploy controller in the configured Kubernetes cluster in ~/.kube/config
# kustomize build config/default | ko apply -f /dev/stdin/

# Generate the default config that includes gcppubsub.
kustomize build config/gcppubsub/ > config/default-gcppubsub.yaml
