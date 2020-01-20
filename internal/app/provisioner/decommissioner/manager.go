/*
 * Copyright 2020 Nalej
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
	"github.com/nalej/provisioner/internal/app/provisioner/provider"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/nalej/provisioner/internal/pkg/workflow"
	"github.com/rs/zerolog/log"
	"sync"
	"time"
)

type Manager struct {
	sync.Mutex
	Config   config.Config
	Executor workflow.Executor
	// Operation per request identifier.
	Operation map[string]entities.InfrastructureOperation
}

func NewManager(config config.Config) Manager {
	return Manager{
		Config:    config,
		Executor:  workflow.GetExecutor(),
		Operation: make(map[string]entities.InfrastructureOperation, 0),
	}
}

func (m *Manager) DecommissionCluster(request *grpc_provisioner_go.DecommissionClusterRequest) (*grpc_common_go.OpResponse, derrors.Error) {
	infraProvider, err := provider.NewInfrastructureProvider(request.TargetPlatform, request.AzureCredentials, &m.Config)
	if err != nil {
		return nil, err
	}
	operation, err := infraProvider.Decommission(entities.NewDecommissionRequest(request))
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("cannot create decommission operation")
		return nil, err
	}
	m.Lock()
	defer m.Unlock()

	_, exists := m.Operation[request.RequestId]
	if exists {
		return nil, derrors.NewAlreadyExistsError("request is already being processed")
	}
	m.Operation[request.RequestId] = operation
	// schedule the operation for execution
	m.Executor.ScheduleOperation(operation)
	// return initial response for the request
	response := &grpc_common_go.OpResponse{
		OrganizationId: request.GetOrganizationId(),
		RequestId:      request.GetRequestId(),
		OperationName:  entities.ToOperationTypeString[entities.Decommission],
		Timestamp:      time.Now().Unix(),
		Status:         grpc_common_go.OpStatus_SCHEDULED,
	}
	return response, nil
}

func (m *Manager) CheckProgress(request *grpc_common_go.RequestId) (*grpc_common_go.OpResponse, derrors.Error) {
	m.Lock()
	defer m.Unlock()
	operation, exists := m.Operation[request.RequestId]
	if !exists {
		return nil, derrors.NewNotFoundError("request_id not found")
	}
	result := operation.Result()
	return result.ToOpResponse()
}

func (m *Manager) RemoveDecommission(request *grpc_common_go.RequestId) (*grpc_common_go.Success, derrors.Error) {
	m.Lock()
	defer m.Unlock()
	_, exists := m.Operation[request.GetRequestId()]
	if !exists {
		return nil, derrors.NewNotFoundError("request_id not found")
	}
	delete(m.Operation, request.GetRequestId())
	return &grpc_common_go.Success{}, nil
}
