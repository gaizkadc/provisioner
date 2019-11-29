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
 *
 */

package management

import (
	"github.com/nalej/derrors"
	grpc_provisioner_go "github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/provisioner/internal/app/provisioner/provider"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/rs/zerolog/log"
	"sync"
	"time"
)

// Manager structure for management operations. This operations are synchronously executed.
type Manager struct {
	sync.Mutex
	Config config.Config
}

func NewManager(config config.Config) Manager {
	return Manager{
		Config: config,
	}
}

type WaitForCompletion struct{
	Called bool
}

func (wfc *WaitForCompletion) finished (requestID string){
	wfc.Called = true
}

// GetKubeConfig retrieves the KubeConfig file to access the management layer of Kubernetes.
// This operation is expected to be executed synchronously.
func (m *Manager) GetKubeConfig(request *grpc_provisioner_go.ClusterRequest) (*grpc_provisioner_go.KubeConfigResponse, derrors.Error) {
	infraProvider, err := provider.NewInfrastructureProvider(request.TargetPlatform, request.AzureCredentials, &m.Config)
	if err != nil {
		return nil, err
	}
	operation, err := infraProvider.GetKubeConfig(entities.NewClusterRequest(request))
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("cannot create get kubeconfig management operation")
		return nil, err
	}
	wfc := &WaitForCompletion{Called:false}
	operation.SetProgress(entities.InProgress)
	operation.Execute(wfc.finished)
	// Wait for the operation to complete within a given deadline.
	// TODO improve with deadline
	for !wfc.Called {
		time.Sleep(5 * time.Second)
	}
	opResult := operation.Result()
	result := &grpc_provisioner_go.KubeConfigResponse{}
	if opResult.Progress == entities.Error{
		result.Error = opResult.ErrorMsg
	}else{
		result.RawKubeConfig = *opResult.KubeConfigResult
	}
	return result, nil
}