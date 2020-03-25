/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package stanutil

import (
	"github.com/nats-io/stan.go"

	"go.uber.org/zap"
)

// Connect creates a new NATS-Streaming connection
func Connect(clusterId string, clientId string, natsUrl string, logger *zap.SugaredLogger) (*stan.Conn, error) {
	logger.Infof("Connect(): clusterId: %v; clientId: %v; natssUrl: %v", clusterId, clientId, natsUrl)
	sc, err := stan.Connect(clusterId, clientId, stan.NatsURL(natsUrl))
	if err != nil {
		logger.Errorf("Connect(): create new connection failed: %v", err)
		return nil, err
	}
	logger.Infof("Connect(): connection to NATSS established, natsConn=%+v", &sc)
	return &sc, nil
}
