module knative.dev/eventing-contrib

go 1.14

require (
	github.com/Shopify/sarama v1.26.4
	github.com/apache/camel-k v0.0.0-20200430164844-778ae8a2bd63
	github.com/aws/aws-sdk-go v1.31.12
	github.com/cloudevents/sdk-go/protocol/kafka_sarama/v2 v2.2.0
	github.com/cloudevents/sdk-go/protocol/stan/v2 v2.2.0
	github.com/cloudevents/sdk-go/v2 v2.2.0
	github.com/davecgh/go-spew v1.1.1
	github.com/flimzy/diff v0.1.7 // indirect
	github.com/go-kivik/couchdb/v3 v3.0.4
	github.com/go-kivik/kivik/v3 v3.0.2
	github.com/go-kivik/kivikmock/v3 v3.0.0
	github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp v0.5.1
	github.com/google/go-github/v31 v31.0.0
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.2
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/nats-io/stan.go v0.6.0
	github.com/otiai10/copy v1.2.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/robfig/cron v1.2.0
	github.com/slinkydeveloper/loadastic v0.0.0-20191203132749-9afe5a010a57
	github.com/stretchr/testify v1.5.1
	github.com/xanzy/go-gitlab v0.32.0
	gitlab.com/flimzy/testy v0.2.1 // indirect
	go.opencensus.io v0.22.5-0.20200716030834-3456e1d174b2
	go.opentelemetry.io/otel v0.4.2 // indirect
	go.uber.org/zap v1.14.1
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	gopkg.in/go-playground/webhooks.v5 v5.13.0
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.18.1
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/utils v0.0.0-20200124190032-861946025e34
	knative.dev/eventing v0.16.1-0.20200729122859-6db549d4b146
	knative.dev/pkg v0.0.0-20200729100859-68b3eeeaab46
	knative.dev/serving v0.16.1-0.20200729045659-06fa35ffc79e
	knative.dev/test-infra v0.0.0-20200729023559-d33097928a54
)

replace (
	github.com/Azure/go-autorest/autorest => github.com/Azure/go-autorest/autorest v0.9.6
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.3.1
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/apiserver => k8s.io/apiserver v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
)
