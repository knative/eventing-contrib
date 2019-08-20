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

source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/e2e-tests.sh

# Environment variables required to build IBM MQ go client
# https://github.com/ibm-messaging/mq-golang#common
export CGO_ENABLED=1
export CGO_LDFLAGS_ALLOW="-Wl,-rpath.*"
export MQ_INSTALLATION_PATH=$(dirname $0)/../contrib/ibm-mq/kodata
export CGO_CFLAGS="-I$MQ_INSTALLATION_PATH/inc"
export CGO_LDFLAGS="-L$MQ_INSTALLATION_PATH/lib64 -Wl,-rpath,$MQ_INSTALLATION_PATH/lib64"