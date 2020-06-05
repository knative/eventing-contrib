module knative.dev/eventing-contrib

go 1.14

require (
	github.com/Shopify/sarama v1.26.4
	github.com/apache/camel-k v0.0.0-20200430164844-778ae8a2bd63
	github.com/aws/aws-sdk-go v1.30.16
	github.com/cloudevents/sdk-go/protocol/kafka_sarama/v2 v2.0.0
	github.com/cloudevents/sdk-go/protocol/stan/v2 v2.0.0
	github.com/cloudevents/sdk-go/v2 v2.0.0
	github.com/davecgh/go-spew v1.1.1
	github.com/flimzy/diff v0.1.7 // indirect
	github.com/go-kivik/couchdb/v3 v3.0.4
	github.com/go-kivik/kivik/v3 v3.0.2
	github.com/go-kivik/kivikmock/v3 v3.0.0
	github.com/golang/protobuf v1.4.0
	github.com/google/go-cmp v0.4.1
	github.com/google/go-github/v31 v31.0.0
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.1
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/nats-io/stan.go v0.6.0
	github.com/otiai10/copy v1.2.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/robfig/cron v1.2.0
	github.com/slinkydeveloper/loadastic v0.0.0-20191203132749-9afe5a010a57
	github.com/xanzy/go-gitlab v0.28.0
	gitlab.com/flimzy/testy v0.2.1 // indirect
	go.opencensus.io v0.22.3
	go.opentelemetry.io/otel v0.4.2 // indirect
	go.uber.org/zap v1.14.1
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	gopkg.in/go-playground/webhooks.v5 v5.13.0
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.17.6
	k8s.io/apimachinery v0.17.6
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/eventing v0.15.1-0.20200605120418-4085d471bd3a
	knative.dev/pkg v0.0.0-20200605112617-7b4093b435c0
	knative.dev/serving v0.15.1-0.20200605022218-74cef7837317
	knative.dev/test-infra v0.0.0-20200605033518-4b5d9e1bbdbe
)

replace (
	github.com/Azure/go-autorest/autorest => github.com/Azure/go-autorest/autorest v0.9.6
	github.com/cloudevents/sdk-go/v2 => github.com/cloudevents/sdk-go/v2 v2.0.0
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.3.1
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/apiserver => k8s.io/apiserver v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
)
