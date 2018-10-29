# This file is needed by kubebuilder but all functionality should exist inside
# the hack/ files.

all: generate manifests test verify
	
# Run tests
test: generate manifests verify
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Deploy default
deploy: manifests
	kustomize build config/default | ko apply -f /dev/stdin

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	./hack/update-manifests.sh

# Generate code
generate: deps
	./hack/update-codegen.sh

# Dep ensure
deps:
	./hack/update-deps.sh

# Verify
verify: verify-codegen verify-manifests

# Verify codegen
verify-codegen:
	./hack/verify-codegen.sh

# Verify manifests
verify-manifests:
	./hack/verify-manifests.sh
