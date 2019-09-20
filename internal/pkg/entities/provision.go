/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package entities

import (
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-installer-go"
	"github.com/nalej/grpc-provisioner-go"
)

// ValidProvisionClusterRequest validates the request to create a new cluster.
func ValidProvisionClusterRequest(request *grpc_provisioner_go.ProvisionClusterRequest) derrors.Error {
	if request.RequestId == "" {
		return derrors.NewInvalidArgumentError("request_id must be set")
	}
	if !request.IsManagementCluster && request.OrganizationId == "" {
		return derrors.NewInvalidArgumentError("organization_id must be set")
	}
	if !request.IsManagementCluster && request.ClusterId == "" {
		return derrors.NewInvalidArgumentError("cluster_id must be set")
	}
	if request.NumNodes <= 0 {
		return derrors.NewInvalidArgumentError("num_nodes must be positive")
	}
	if request.NodeType == "" {
		return derrors.NewInvalidArgumentError("node_type must be set")
	}
	if request.TargetPlatform == grpc_installer_go.Platform_AZURE && request.AzureCredentials == nil {
		return derrors.NewInvalidArgumentError("azure_credentials must be set when type is Azure")
	}
	return nil
}

type ProvisionRequest struct {
	// RequestID with the request identifier.
	RequestID string
	// OrganizationId with the organization identifier.
	OrganizationID string
	// ClusterId with the cluster identifier.
	ClusterID string
	// NumNodes with the number of nodes of the cluster to be created.
	NumNodes int64
	// NodeType with the type of node to be used. This value must exist in the target infrastructure provider.
	NodeType string
	// Zone where the cluster will be provisioned. This value must exist in the target infrastructure provider.
	Zone string
	// IsManagementCluster to determine if the provisioning is for a management or application cluster.
	IsManagementCluster bool
}

func NewProvisionRequest(request *grpc_provisioner_go.ProvisionClusterRequest) ProvisionRequest {
	return ProvisionRequest{
		RequestID:           request.RequestId,
		OrganizationID:      request.OrganizationId,
		ClusterID:           request.ClusterId,
		NumNodes:            request.NumNodes,
		NodeType:            request.NodeType,
		Zone:                request.Zone,
		IsManagementCluster: request.IsManagementCluster,
	}
}

type ScaleRequest struct {
	// NumNodes with the number of nodes of the cluster to be scaled to.
	NumNodes int
	// NodeType with the type of node to be used. This value must exist in the target infrastructure provider.
	NodeType string
	// Zone where the cluster will be provisioned. This value must exist in the target infrastructure provider.
	Zone string
}

// ProvisionResult with the result of a successful provisioning.
type ProvisionResult struct {
	// RawKubeConfig contains the contents of the resulting kubeconfig files
	RawKubeConfig string
}
