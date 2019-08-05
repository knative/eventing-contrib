source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/e2e-tests.sh

initialize $@ --skip-istio-addon

echo "current path: $(pwd)"

echo "GOPATH: ${GOPATH}"

cd ${GOPATH} && mkdir -p src/github.com && cd src/github.com
git clone https://github.com/knative/eventing
cd knative/eventing
ko apply -f config/
