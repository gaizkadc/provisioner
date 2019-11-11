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

package azure

import (
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-provisioner-go"
	providerEntities "github.com/nalej/provisioner/internal/app/provisioner/provider/entities"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/rs/zerolog/log"
)

type AzureInfrastructureProvider struct {
	credentials *AzureCredentials
	config      *config.Config
}

func NewAzureInfrastructureProvider(credentials *grpc_provisioner_go.AzureCredentials, config *config.Config) (providerEntities.InfrastructureProvider, derrors.Error) {
	creds := NewAzureCredentials(credentials)
	return &AzureInfrastructureProvider{creds, config}, nil
}

func (aip *AzureInfrastructureProvider) Provision(request entities.ProvisionRequest) (entities.InfrastructureOperation, derrors.Error) {
	// TODO remove this log entry
	log.Debug().Interface("credentials", aip.credentials).Interface("request", request).Msg("creating provision operation")
	return NewProvisionerOperation(aip.credentials, request, aip.config)
}

func (aip *AzureInfrastructureProvider) Decomission() (entities.InfrastructureOperation, derrors.Error) {
	panic("implement me")
}

func (aip *AzureInfrastructureProvider) Scale(request entities.ScaleRequest) (entities.InfrastructureOperation, derrors.Error) {
	panic("implement me")
}
