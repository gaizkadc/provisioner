/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package provider

import (
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-installer-go"
	"github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/provisioner/internal/app/provisioner/provider/azure"
	"github.com/nalej/provisioner/internal/app/provisioner/provider/entities"
	"github.com/rs/zerolog/log"
)

// NewInfrastructureProvider creates a new provider for a given target platform. Extra parameters are optional depending
// on the type of provider to be created.
func NewInfrastructureProvider(targetPlaform grpc_installer_go.Platform, azureCredentials *grpc_provisioner_go.AzureCredentials) (entities.InfrastructureProvider, derrors.Error){
	switch targetPlaform {
	case grpc_installer_go.Platform_AZURE:
		return azure.NewAzureInfrastructureProvider(azureCredentials), nil
	}
	log.Debug().Str("targetPlatform", targetPlaform.String()).Msg("unsupported target platform for creating a provider")
	return nil, derrors.NewUnimplementedError("unsupported target platform for creating a provider").WithParams(targetPlaform.String())
}

