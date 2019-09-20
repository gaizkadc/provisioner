/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package azure

import (
	"github.com/Azure/go-autorest/autorest"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-provisioner-go"
	providerEntities "github.com/nalej/provisioner/internal/app/provisioner/provider/entities"
	"github.com/nalej/provisioner/internal/pkg/entities"
)

type AzureInfrastructureProvider struct {
	credentials *AzureCredentials
	authorizer  autorest.Authorizer
}

func NewAzureInfrastructureProvider(credentials *grpc_provisioner_go.AzureCredentials) (providerEntities.InfrastructureProvider, derrors.Error) {
	creds := NewAzureCredentials(credentials)
	// create an authorizer from env vars or Azure Managed Service Idenity
	authorizer, err := GetAuthorizer(creds)
	if err != nil {
		return nil, err
	}
	return &AzureInfrastructureProvider{creds, authorizer}, nil
}

func (aip *AzureInfrastructureProvider) Provision(request entities.ProvisionRequest) (entities.InfrastructureOperation, derrors.Error) {
	return NewProvisionerOperation(aip.credentials, aip.authorizer, request), nil
}

func (aip *AzureInfrastructureProvider) Decomission() (entities.InfrastructureOperation, derrors.Error) {
	panic("implement me")
}

func (aip *AzureInfrastructureProvider) Scale(request entities.ScaleRequest) (entities.InfrastructureOperation, derrors.Error) {
	panic("implement me")
}
