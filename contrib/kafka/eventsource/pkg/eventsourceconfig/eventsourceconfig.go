package eventsourceconfig

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type eventsourceconfig struct {
	BootStrapServers              string
	KafkaTopic                    string
	ConsumerGroupID               string
	KafkaVersion                  string
	NetMaxOpenRequests            int64
	NetKeepAlive                  int64
	NetSaslEnable                 bool
	NetSaslHandshake              bool
	NetSaslUser                   string
	NetSaslPassword               string
	ConsumerMaxWaitTime           int64
	ConsumerMaxProcessingTime     int64
	ConsumerOffsetsCommitInterval int64
	ConsumerOffsetsInitial        string
	ConsumerOffsetsRetention      int64
	ConsumerOffsetsRetryMax       int64
	ChannelBufferSize             int64
	GroupPartitionStrategy        string
	GroupSessionTimeout           int64

	Target string
}

//GetConfig gets the initial config
func GetConfig() eventsourceconfig {

	return eventsourceconfig{
		BootStrapServers:              strings.ToLower(getStrEnv("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")),
		KafkaTopic:                    os.Getenv("KAFKA_TOPIC"),
		ConsumerGroupID:               os.Getenv("CONSUMER_GROUP_ID"),
		KafkaVersion:                  getStrEnv("KAFKA_VERSION", "2.0.0"),
		NetMaxOpenRequests:            getIntEnv("NET_MAX_OPEN_REQUESTS", 5),
		NetKeepAlive:                  getIntEnv("NET_KEEPALIVE", 0),
		NetSaslEnable:                 getBoolEnv("SASL_ENABLE", false),
		NetSaslHandshake:              getBoolEnv("SASL_HANDSHAKE", false),
		NetSaslUser:                   os.Getenv("SASL_USER"),
		NetSaslPassword:               os.Getenv("SASL_PASSWORD"),
		ConsumerMaxWaitTime:           getIntEnv("CONSUMER_MAX_WAIT_TIME", 250000000),
		ConsumerMaxProcessingTime:     getIntEnv("CONSUMER_MAX_PROCESSING_TIME", 100000000),
		ConsumerOffsetsCommitInterval: getIntEnv("CONSUMER_OFFSETS_COMMIT_INTERVAL", 1000000000),
		ConsumerOffsetsInitial:        getStrEnv("CONSUMER_OFFSETS_INITIAL", "OffsetNewest"),
		ConsumerOffsetsRetention:      getIntEnv("CONSUMER_OFFSETS_RETENTION", 0),
		ConsumerOffsetsRetryMax:       getIntEnv("CONSUMER_OFFSETS_RETRY_MAX", 3),
		ChannelBufferSize:             getIntEnv("CHANNEL_BUFFER_SIZE", 256),
		GroupPartitionStrategy:        getStrEnv("GROUP_PARTITION_STRATEGY", "Range"),
		GroupSessionTimeout:           getIntEnv("GROUP_SESSION_TIMEOUT", 30000000000),

		Target: os.Getenv("TARGET"),
	}
}

func getStrEnv(key string, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getIntEnv(key string, defaultVal int64) int64 {
	if val := os.Getenv(key); val != "" {
		intval, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			log.Printf("Error parsing int environment variable: %s -> %s", key, val)
		}
		return intval
	}
	return defaultVal
}

func getBoolEnv(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		boolval, err := strconv.ParseBool(val)
		if err != nil {
			log.Printf("Error parsing bool environment variable: %s -> %s", key, val)
		}
		return boolval
	}
	return defaultVal
}
