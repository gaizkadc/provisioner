/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package azure

import "github.com/nalej/grpc-provisioner-go"

type AzureCredentials struct {
	ClientId                       string
	ClientSecret                   string
	SubscriptionId                 string
	TenantId                       string
	ActiveDirectoryEndpointUrl     string
	ResourceManagerEndpointUrl     string
	ActiveDirectoryGraphResourceId string
	SqlManagementEndpointUrl       string
	GalleryEndpointUrl             string
	ManagementEndpointUrl          string
}

func NewAzureCredentials(credentials *grpc_provisioner_go.AzureCredentials) *AzureCredentials {
	return &AzureCredentials{
		ClientId:                       credentials.ClientId,
		ClientSecret:                   credentials.ClientSecret,
		SubscriptionId:                 credentials.SubscriptionId,
		TenantId:                       credentials.TenantId,
		ActiveDirectoryEndpointUrl:     credentials.ActiveDirectoryEndpointUrl,
		ResourceManagerEndpointUrl:     credentials.ResourceManagerEndpointUrl,
		ActiveDirectoryGraphResourceId: credentials.ActiveDirectoryEndpointUrl,
		SqlManagementEndpointUrl:       credentials.SqlManagementEndpointUrl,
		GalleryEndpointUrl:             credentials.GalleryEndpointUrl,
		ManagementEndpointUrl:          credentials.ManagementEndpointUrl,
	}
}
