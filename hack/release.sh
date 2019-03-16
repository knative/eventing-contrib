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

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/release.sh

# Yaml files to generate, and the source config dir for them.
declare -A COMPONENTS
COMPONENTS=(
  ["eventing-sources.yaml"]="config"
  ["gcppubsub.yaml"]="contrib/gcppubsub/config"
  ["event-display.yaml"]="config/tools/event-display"
  ["camel.yaml"]="contrib/camel/config"
  ["kafka.yaml"]="contrib/kafka/config"
  ["awssqs.yaml"]="contrib/awssqs/config"
)
readonly COMPONENTS

function build_release() {
  local all_yamls=()
  for yaml in "${!COMPONENTS[@]}"; do
  local config="${COMPONENTS[${yaml}]}"
    echo "Building Knative Eventing Sources - ${config}"
    ko resolve ${KO_FLAGS} -f ${config}/ > ${yaml}
    all_yamls+=(${yaml})
  done
  YAMLS_TO_PUBLISH="${all_yamls[@]}"
}

main $@
