module knative.dev/eventing-contrib

go 1.13

require (
	contrib.go.opencensus.io/exporter/zipkin v0.1.1 // indirect
	github.com/Shopify/sarama v1.24.0
	github.com/apache/camel-k v0.0.0-20200224174830-24ddce5afb41
	github.com/aws/aws-sdk-go v1.27.1
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cloudevents/sdk-go v1.2.0
	github.com/cloudevents/sdk-go/v2 v2.0.0-preview8
	github.com/davecgh/go-spew v1.1.1
	github.com/eapache/go-resiliency v1.2.0 // indirect
	github.com/flimzy/diff v0.1.7 // indirect
	github.com/flimzy/testy v0.1.17 // indirect
	github.com/go-kivik/couchdb v2.0.0-pre3.0.20190830175249-a8dab644dca9+incompatible
	github.com/go-kivik/kivik v2.0.0-pre2.0.20191021113424-7eea98010d9d+incompatible
	github.com/go-kivik/kivikmock v2.0.0-pre3+incompatible
	github.com/go-kivik/kiviktest v2.0.0+incompatible // indirect
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.4.0
	github.com/google/go-containerregistry v0.0.0-20200331213917-3d03ed9b1ca2 // indirect
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.1
	github.com/grpc-ecosystem/grpc-gateway v1.12.1 // indirect
	github.com/influxdata/tdigest v0.0.1 // indirect
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/klauspost/compress v1.9.1 // indirect
	github.com/nats-io/nats-server/v2 v2.1.6 // indirect
	github.com/nats-io/stan.go v0.6.0
	github.com/pierrec/lz4 v2.3.0+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.7.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20190826022208-cac0b30c2563 // indirect
	github.com/robfig/cron v1.2.0
	github.com/slinkydeveloper/loadastic v0.0.0-20191203132749-9afe5a010a57
	github.com/xanzy/go-gitlab v0.28.0
	gitlab.com/flimzy/testy v0.2.1 // indirect
	go.opencensus.io v0.22.3
	go.opentelemetry.io/otel v0.4.2 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/exp v0.0.0-20191029154019-8994fa331a53 // indirect
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	gopkg.in/go-playground/webhooks.v5 v5.13.0
	gopkg.in/jcmturner/gokrb5.v7 v7.3.0 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20200427153329-656914f816f9 // indirect
	knative.dev/eventing v0.14.1-0.20200427112650-40f0a540923e
	knative.dev/pkg v0.0.0-20200427190051-6b9ee63b4aad
	knative.dev/serving v0.14.1-0.20200426043050-7ad5cc721f86
)

replace (
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	// most of these kube replaces are because of camel-k
	k8s.io/api => k8s.io/api v0.16.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.16.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.4
	k8s.io/apiserver => k8s.io/apiserver v0.16.4
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.16.4
	k8s.io/client-go => k8s.io/client-go v0.16.4
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.16.4
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.16.4
	k8s.io/code-generator => k8s.io/code-generator v0.16.4
	k8s.io/component-base => k8s.io/component-base v0.16.4
	k8s.io/cri-api => k8s.io/cri-api v0.16.4
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.16.4
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.16.4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.16.4
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.16.4
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.16.4
	k8s.io/kubectl => k8s.io/kubectl v0.16.4
	k8s.io/kubelet => k8s.io/kubelet v0.16.4
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.16.4
	k8s.io/metrics => k8s.io/metrics v0.16.4
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.16.4
	knative.dev/eventing => github.com/grantr/eventing v0.0.0-20200423234910-75c3a31a9a86
)

// because of camel-k
replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by Helm
