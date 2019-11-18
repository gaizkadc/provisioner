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

package provisioner

import (
	"sync"

	"github.com/nalej/derrors"
	"github.com/nalej/grpc-common-go"
	"github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/provisioner/internal/app/provisioner/provider"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/nalej/provisioner/internal/pkg/workflow"
	"github.com/rs/zerolog/log"
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

// ProvisionCluster triggers the provisioning operation on a given cloud infrastructure provider.
func (m *Manager) ProvisionCluster(request *grpc_provisioner_go.ProvisionClusterRequest) (*grpc_provisioner_go.ProvisionClusterResponse, derrors.Error) {
	log.Debug().Str("requestID", request.RequestId).
		Str("target_platform", request.TargetPlatform.String()).Msg("Provision request received")
	infraProvider, err := provider.NewInfrastructureProvider(request.TargetPlatform, request.AzureCredentials, &m.Config)
	if err != nil {
		return nil, err
	}
	operation, err := infraProvider.Provision(entities.NewProvisionRequest(request))
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("cannot create provision operation")
		return nil, err
	}
	m.Lock()
	defer m.Unlock()
	// Check if the operation is already registered
	_, exists := m.Operation[request.RequestId]
	if exists {
		return nil, derrors.NewAlreadyExistsError("request is already being processed")
	}
	m.Operation[request.RequestId] = operation
	// schedule the operation for execution
	m.Executor.ScheduleOperation(operation)
	// return initial response for the request
	response := &grpc_provisioner_go.ProvisionClusterResponse{
		RequestId:   request.RequestId,
		State:       grpc_provisioner_go.ProvisionProgress_INIT,
		ElapsedTime: 0,
		Error:       "",
		ClusterName: request.ClusterName,
	}
	return response, nil
}

// CheckProgress gets an updated state of a provisioning request.
func (m *Manager) CheckProgress(requestID *grpc_common_go.RequestId) (*grpc_provisioner_go.ProvisionClusterResponse, derrors.Error) {
	m.Lock()
	defer m.Unlock()
	operation, exists := m.Operation[requestID.RequestId]
	if !exists {
		return nil, derrors.NewNotFoundError("request_id not found")
	}
	result := operation.Result()
	return result.ToProvisionClusterResult()
}

// RemoveProvision cancels an ongoing provisioning or removes the information of an already processed provision operation.
func (m *Manager) RemoveProvision(requestID *grpc_common_go.RequestId) derrors.Error {
	m.Lock()
	defer m.Unlock()
	_, exists := m.Operation[requestID.RequestId]
	if !exists {
		return derrors.NewNotFoundError("request_id not found")
	}
	delete(m.Operation, requestID.RequestId)
	return nil
}
