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

source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/library.sh

cd ${REPO_ROOT_DIR}

# The list of dependencies that we track at HEAD and periodically
# float forward in this repository.
FLOATING_DEPS=(
  "knative.dev/pkg"
  "knative.dev/eventing"
  "knative.dev/serving"
  "knative.dev/test-infra"
)

# Parse flags to determine any we should pass to dep.
DEP_FLAGS=()
while [[ $# -ne 0 ]]; do
  parameter=$1
  case ${parameter} in
    --upgrade) DEP_FLAGS=( -update ${FLOATING_DEPS[@]} ) ;;
    *) abort "unknown option ${parameter}" ;;
  esac
  shift
done
readonly DEP_FLAGS

# Ensure we have everything we need under vendor/
dep ensure ${DEP_FLAGS[@]}

rm -rf $(find vendor/ -name 'OWNERS')
rm -rf $(find vendor/ -name 'OWNERS_ALIASES')
rm -rf $(find vendor/ -name 'BUILD')
rm -rf $(find vendor/ -name 'BUILD.bazel')

update_licenses third_party/VENDOR-LICENSE "./cmd/*" "./github/cmd/*" "./camel/source/cmd/*" \
		"./kafka/source/cmd/*" "./kafka/channel/cmd/*" "./awssqs/cmd/*" "./gitlab/cmd/*" \
		"./natss/cmd/*" "./couchdb/source/cmd/*" "./ceph/cmd/*"

# HACK HACK HACK
# The only way we found to create a consistent Trace tree without any missing Spans is to
# artificially set the SpanId. See pkg/tracing/traceparent.go for more details.
# Produced with:
# git diff origin/master HEAD -- vendor/go.opencensus.io/trace/trace.go > ./hack/set-span-id.patch
git apply ${REPO_ROOT_DIR}/hack/set-span-id.patch

# Patch Kivik
# see https://github.com/go-kivik/kivik/issues/420
git apply ${REPO_ROOT_DIR}/hack/kivik-set-zero.patch

## Hack to vendor performance image from eventing
rm -rf ${REPO_ROOT_DIR}/vendor/knative.dev/eventing/test/test_images/performance/kodata/*
ln -s ../../../../../../../.git/HEAD ${REPO_ROOT_DIR}/vendor/knative.dev/eventing/test/test_images/performance/kodata/HEAD
ln -s ../../../../../../../.git/refs ${REPO_ROOT_DIR}/vendor/knative.dev/eventing/test/test_images/performance/kodata/refs
