/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package azure

import (
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-provisioner-go"
	"math"
)

// AzureCredentials contains the set of values required to interact with the Azure SDK.
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

// NewAzureCredentials creates a new credentials from the ones received from gRPC.
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

// Int64ToInt32 casts an int64 value to int32 if it does not overflow.
func Int64ToInt32(value int64) (*int32, derrors.Error) {
	if value > math.MaxInt32 || value < math.MinInt32 {
		return nil, derrors.NewInvalidArgumentError("integer overflow")
	}
	toInt32 := int32(value)
	return &toInt32, nil
}

// Int64AsPTR returns a pointer to a given int64 value.
func Int64AsPTR(value int64) *int64 {
	return &value
}

// Int32AsPTR returns a pointer to a given int32 value.
func Int32AsPTR(value int32) *int32 {
	return &value
}

// StringAsPTR returns a pointer to a given string value.
func StringAsPTR(value string) *string {
	return &value
}

// BoolAsPTR returns a pointer to a given bool value.
func BoolAsPTR(value bool) *bool {
	return &value
}
