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

# Documentation about this script and how to use it can be found
# at https://github.com/knative/test-infra/tree/master/ci

source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/release.sh

# Yaml files to generate, and the source config dir for them.
declare -A COMPONENTS
COMPONENTS=(
  ["appender.yaml"]="config/tools/appender"
  ["awssqs.yaml"]="awssqs/config"
  ["camel.yaml"]="camel/source/config"
  ["couchdb.yaml"]="couchdb/source/config"
  ["event-display.yaml"]="config/tools/event-display"
  ["github.yaml"]="github/config"
  ["kafka-source.yaml"]="kafka/source/config"
  ["kafka-channel.yaml"]="kafka/channel/config"
  ["natss-channel.yaml"]="natss/config"
  ["prometheus-source.yaml"]="prometheus/config"
)
readonly COMPONENTS

function build_release() {
   # Update release labels if this is a tagged release
  if [[ -n "${TAG}" ]]; then
    echo "Tagged release, updating release labels to contrib.eventing.knative.dev/release: \"${TAG}\""
    LABEL_YAML_CMD=(sed -e "s|contrib.eventing.knative.dev/release: devel|contrib.eventing.knative.dev/release: \"${TAG}\"|")
  else
    echo "Untagged release, will NOT update release labels"
    LABEL_YAML_CMD=(cat)
  fi

  local all_yamls=()
  for yaml in "${!COMPONENTS[@]}"; do
    local config="${COMPONENTS[${yaml}]}"
    echo "Building Knative Eventing Contrib - ${config}"
    # TODO(chizhg): reenable --strict mode after https://github.com/knative/test-infra/issues/1262 is fixed.
    ko resolve ${KO_FLAGS} -f ${config}/ | "${LABEL_YAML_CMD[@]}" > ${yaml}
    all_yamls+=(${yaml})
  done
  ARTIFACTS_TO_PUBLISH="${all_yamls[@]}"
}

main $@
