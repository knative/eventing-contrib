source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/e2e-tests.sh

function knative_setup() {
  echo "current path: $(pwd)"
  echo "GOPATH: ${GOPATH}"

  pushd .
  cd ${GOPATH} && mkdir -p src/github.com && cd src/github.com
  git clone https://github.com/knative/eventing
  cd ${GOPATH}/src/github.com/knative/eventing
  ko apply -f config/
  popd
}

initialize $@ --skip-istio-addon