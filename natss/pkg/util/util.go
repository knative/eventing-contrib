package util

import (
	"fmt"
	"os"

	eventingutils "knative.dev/eventing/pkg/utils"
)

const (
	// DefaultNatssURLKey is the environment variable that can be set to specify the natss url
	defaultNatssURLVar  = "DEFAULT_NATSS_URL"
	defaultClusterIDVar = "DEFAULT_CLUSTER_ID"

	fallbackDefaultNatssURLTmpl = "nats://nats-streaming.natss.svc.%s:4222"
	fallbackDefaultClusterID    = "knative-nats-streaming"

	defaultMaxIdleConnections        = 1000
	defaultMaxIdleConnectionsPerHost = 100
)

// GetDefaultNatssURL returns the default natss url to connect to
func GetDefaultNatssURL() string {
	return getEnv(defaultNatssURLVar, fmt.Sprintf(fallbackDefaultNatssURLTmpl, eventingutils.GetClusterDomainName()))
}

// GetDefaultClusterID returns the default cluster id to connect with
func GetDefaultClusterID() string {
	return getEnv(defaultClusterIDVar, fallbackDefaultClusterID)
}

// GetMaxIdleConnections returns the max number of idle connections
func GetMaxIdleConnections() int {
	return defaultMaxIdleConnections
}

// GetMaxIdleConnections returns the max number of idle connections per host
func GetMaxIdleConnectionsPerHost() int {
	return defaultMaxIdleConnectionsPerHost
}

func getEnv(envKey string, fallback string) string {
	val, ok := os.LookupEnv(envKey)
	if !ok {
		return fallback
	}
	return val
}
