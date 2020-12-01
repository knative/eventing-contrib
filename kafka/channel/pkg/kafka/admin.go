/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kafka

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/Shopify/sarama"
	"knative.dev/pkg/logging"
)

var mutex sync.Mutex

type ClusterAdminFactory func() (sarama.ClusterAdmin, error)

type AdminClient interface {
	// ListConsumerGroups Lists the consumer groups
	ListConsumerGroups() ([]string, error)
}

// AdminClientManager manages a ClusterAdmin connection and recreates one when needed
// it is made to overcome https://github.com/Shopify/sarama/issues/1162
type AdminClientManager struct {
	logger       *zap.SugaredLogger
	adminFactory ClusterAdminFactory
	clusterAdmin sarama.ClusterAdmin
}

func NewAdminClient(ctx context.Context, caFactory ClusterAdminFactory) (AdminClient, error) {
	logger := logging.FromContext(ctx)
	logger.Info("Creating a new AdminClient")
	kafkaClusterAdmin, err := caFactory()
	if err != nil {
		logger.Errorw("error while creating ClusterAdmin", zap.Error(err))
		return nil, err
	}
	return &AdminClientManager{
		logger:       logger,
		adminFactory: caFactory,
		clusterAdmin: kafkaClusterAdmin,
	}, nil
}

// ListConsumerGroups Returns a list of the consumer groups.
//
// In the occasion of errors, there will be a retry with an exponential backoff.
// Due to a known issue in Sarama ClusterAdmin https://github.com/Shopify/sarama/issues/1162,
// a new ClusterAdmin will be created with every retry until the call succeeds or
// the timeout is reached.
func (c *AdminClientManager) ListConsumerGroups() ([]string, error) {
	c.logger.Info("Attempting to list consumer group")
	mutex.Lock()
	defer mutex.Unlock()
	r := 0
	// This gives us around ~13min of exponential backoff
	max := 13
	cgsMap, err := c.clusterAdmin.ListConsumerGroups()
	for err != nil && r <= max {
		// There's on error, let's retry and presume a new ClusterAdmin can fix it

		// Calculate incremental delay following this https://docs.aws.amazon.com/general/latest/gr/api-retries.html
		t := int(math.Pow(2, float64(r)) * 100)
		d := time.Duration(t) * time.Millisecond
		c.logger.Errorw("listing consumer group failed. Refreshing the ClusterAdmin and retrying.",
			zap.Error(err),
			zap.Duration("retry after", d),
			zap.Int("Retry attempt", r),
			zap.Int("Max retries", max),
		)
		time.Sleep(d)

		// let's reconnect and try again
		c.clusterAdmin, err = c.adminFactory()
		r += 1
		if err != nil {
			// skip this attempt
			continue
		}
		cgsMap, err = c.clusterAdmin.ListConsumerGroups()
	}

	if r > max {
		return nil, fmt.Errorf("failed to refresh the culster admin and retry: %v", err)
	}

	return sets.StringKeySet(cgsMap).List(), nil
}
