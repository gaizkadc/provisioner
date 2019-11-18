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

package scaler

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

// ScaleCluster triggers the rescaling of a given cluster by adding or removing nodes.
func (m *Manager) ScaleCluster(request *grpc_provisioner_go.ScaleClusterRequest) (*grpc_provisioner_go.ScaleClusterResponse, error) {
	infraProvider, err := provider.NewInfrastructureProvider(request.TargetPlatform, request.AzureCredentials, &m.Config)
	if err != nil {
		return nil, err
	}
	operation, err := infraProvider.Scale(entities.NewScaleRequest(request))
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("cannot create scale operation")
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
	response := &grpc_provisioner_go.ScaleClusterResponse{
		RequestId:   request.RequestId,
		State:       grpc_provisioner_go.ProvisionProgress_INIT,
		ElapsedTime: 0,
		Error:       "",
	}
	return response, nil
}

// CheckProgress gets an updated state of a scale request.
func (m *Manager) CheckProgress(requestID *grpc_common_go.RequestId) (*grpc_provisioner_go.ScaleClusterResponse, error) {
	m.Lock()
	defer m.Unlock()
	operation, exists := m.Operation[requestID.RequestId]
	if !exists {
		return nil, derrors.NewNotFoundError("request_id not found")
	}
	result := operation.Result()
	return result.ToScaleClusterResult()
}

// RemoveScale cancels an ongoing scale process or removes the information of an already processed one.
func (m *Manager) RemoveScale(requestID *grpc_common_go.RequestId) derrors.Error {
	m.Lock()
	defer m.Unlock()
	_, exists := m.Operation[requestID.RequestId]
	if !exists {
		return derrors.NewNotFoundError("request_id not found")
	}
	delete(m.Operation, requestID.RequestId)
	return nil
}
