# © Copyright IBM Corporation 2015, 2018
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM ubuntu:16.04

LABEL maintainer "Riccardo Biraghi <riccardo.biraghi@ibm.com>"

# The URL to download the MQ demo from in tar.gz format
ARG DEMO_URL=https://public.dhe.ibm.com/ibmdl/export/pub/software/websphere/messaging/mqdev/demo/mq-demo-lin.tar.gz

RUN export DEBIAN_FRONTEND=noninteractive \
  # Install additional packages required by MQ, this install process and the runtime scripts
  && apt-get update -y \
  && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    file \
    tar \
  # Download and extract the MQ demo
  && export DIR_EXTRACT=/tmp/demo \
  && mkdir -p ${DIR_EXTRACT} \
  && cd ${DIR_EXTRACT} \
  && curl -LO $DEMO_URL \
  && tar -zxvf ./*.tar.gz \
  # Recommended: Remove packages only needed by this script
  && apt-get purge -y \
    ca-certificates \
    curl \
  && ln -s ${DIR_EXTRACT}/mq-demo /usr/local/bin/mq-demo

ENV LANG=en_US.UTF-8

ENTRYPOINT ["mq-demo"]
