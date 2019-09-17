/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package azure

import (
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-provisioner-go"
	providerEntities "github.com/nalej/provisioner/internal/app/provisioner/provider/entities"
	"github.com/nalej/provisioner/internal/pkg/entities"
)

type AzureInfrastructureProvider struct{
	credentials * AzureCredentials
}

func NewAzureInfrastructureProvider(credentials *grpc_provisioner_go.AzureCredentials) providerEntities.InfrastructureProvider {
	creds := NewAzureCredentials(credentials)
	return &AzureInfrastructureProvider{creds}
}

func (aip *AzureInfrastructureProvider) Provision(request entities.ProvisionRequest) (entities.InfrastructureOperation, derrors.Error) {
	panic("implement me")
}

func (aip *AzureInfrastructureProvider) Decomission() (entities.InfrastructureOperation, derrors.Error) {
	panic("implement me")
}

func (aip *AzureInfrastructureProvider) Scale(request entities.ScaleRequest) (entities.InfrastructureOperation, derrors.Error) {
	panic("implement me")
}



