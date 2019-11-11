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

package provider

import (
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-installer-go"
	"github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/provisioner/internal/app/provisioner/provider/azure"
	"github.com/nalej/provisioner/internal/app/provisioner/provider/entities"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/rs/zerolog/log"
)

// NewInfrastructureProvider creates a new provider for a given target platform. Extra parameters are optional depending
// on the type of provider to be created.
func NewInfrastructureProvider(targetPlaform grpc_installer_go.Platform, azureCredentials *grpc_provisioner_go.AzureCredentials, config *config.Config) (entities.InfrastructureProvider, derrors.Error) {
	switch targetPlaform {
	case grpc_installer_go.Platform_AZURE:
		return azure.NewAzureInfrastructureProvider(azureCredentials, config)
	}
	log.Debug().Str("targetPlatform", targetPlaform.String()).Msg("unsupported target platform for creating a provider")
	return nil, derrors.NewUnimplementedError("unsupported target platform for creating a provider").WithParams(targetPlaform.String())
}
