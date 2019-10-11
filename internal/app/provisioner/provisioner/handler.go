/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package provisioner

import (
	grpc_common_go "github.com/nalej/grpc-common-go"
	grpc_provisioner_go "github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/grpc-utils/pkg/conversions"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/context"
)

type Handler struct {
	Manager Manager
}

func NewHandler(manager Manager) *Handler {
	return &Handler{manager}
}

// ProvisionCluster triggers the provisioning operation on a given cloud infrastructure provider.
func (h *Handler) ProvisionCluster(ctx context.Context, request *grpc_provisioner_go.ProvisionClusterRequest) (*grpc_provisioner_go.ProvisionClusterResponse, error) {
	err := entities.ValidProvisionClusterRequest(request)
	if err != nil {
		log.Warn().Str("trace", err.DebugReport()).Msg(err.Error())
		return nil, conversions.ToGRPCError(err)
	}
	return h.Manager.ProvisionCluster(request)
}

// CheckProgress gets an updated state of a provisioning request.
func (h *Handler) CheckProgress(ctx context.Context, requestID *grpc_common_go.RequestId) (*grpc_provisioner_go.ProvisionClusterResponse, error) {
	return h.Manager.CheckProgress(requestID)
}

// RemoveProvision cancels an ongoing provisioning or removes the information of an already processed provision operation.
func (h *Handler) RemoveProvision(ctx context.Context, requestID *grpc_common_go.RequestId) (*grpc_common_go.Success, error) {
	err := h.Manager.RemoveProvision(requestID)
	if err != nil{
		return nil, err
	}
	return &grpc_common_go.Success{}, nil
}
