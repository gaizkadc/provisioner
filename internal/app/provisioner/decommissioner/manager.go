/*
 * Copyright 2019 Nalej
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

package decommissioner

import (
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-common-go"
	"github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/nalej/provisioner/internal/pkg/workflow"
	"sync"
)

type Manager struct {
	sync.Mutex
	Config config.Config
	Executor workflow.Executor
	// Operation per request identifier.
	Operation map[string]entities.InfrastructureOperation
}

func NewManager(config config.Config) Manager {
	return Manager{
		Config: config,
		Executor:  workflow.GetExecutor(),
		Operation: make(map[string]entities.InfrastructureOperation, 0),
	}
}

func (m *Manager) DecommissionCluster(request *grpc_provisioner_go.DecomissionClusterRequest) (*grpc_common_go.OpResponse, derrors.Error) {
	panic("implement me")
}

func (m *Manager) CheckProgress(request *grpc_common_go.RequestId) (*grpc_common_go.OpResponse, derrors.Error) {
	panic("implement me")
}

func (m *Manager) RemoveDecommission(request *grpc_common_go.RequestId) (*grpc_common_go.Success, derrors.Error) {
	panic("implement me")
}