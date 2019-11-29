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

package entities

import (
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-installer-go"
	"github.com/nalej/grpc-provisioner-go"
)

// ManagementOperationType defines an enumeration of the supported operations.
type ManagementOperationType int
const (
	// GetKubeConfig retrieves the kubeconfig associated with a cluster.
	GetKubeConfig ManagementOperationType = iota + 1
)

// ClusterRequest message with the information required to process a management request.
type ClusterRequest struct {
	// RequestID generated by the infrastructure-manager component to track the provisioning operation.
	RequestID string
	// OrganizationID with the organization identifier.
	OrganizationID string
	// ClusterID with the cluster identifier.
	ClusterID string
	// IsManagementCluster to determine if the provisioning is for a management or application cluster.
	IsManagementCluster bool
	// AzureOptions with the provisioning specific options.
	AzureOptions         *AzureOptions
}

func NewClusterRequest(request *grpc_provisioner_go.ClusterRequest) ClusterRequest{
	return ClusterRequest{
		RequestID:           request.RequestId,
		OrganizationID:      request.OrganizationId,
		ClusterID:           request.ClusterId,
		IsManagementCluster: request.IsManagementCluster,
		AzureOptions:        NewAzureOptions(request.AzureOptions),
	}
}

func ValidClusterRequest(request *grpc_provisioner_go.ClusterRequest) derrors.Error{
	if request.RequestId == "" {
		return derrors.NewInvalidArgumentError("request_id must be set by infrastructure-manager")
	}
	if request.OrganizationId == "" {
		return derrors.NewInvalidArgumentError("organization_id cannot be empty")
	}
	if request.ClusterId == "" {
		return derrors.NewInvalidArgumentError("cluster_id cannot be empty")
	}
	if request.TargetPlatform == grpc_installer_go.Platform_AZURE && (request.AzureOptions == nil || request.AzureOptions.ResourceGroup == "") {
		return derrors.NewInvalidArgumentError("azure_options.resource_group cannot be empty")
	}
	return nil
}