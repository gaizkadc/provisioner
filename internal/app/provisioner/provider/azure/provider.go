/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
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
