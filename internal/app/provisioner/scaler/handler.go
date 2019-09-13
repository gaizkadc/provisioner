/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package scaler

import (
	"github.com/nalej/grpc-common-go"
	"github.com/nalej/grpc-provisioner-go"
	"golang.org/x/net/context"
)

type Handler struct {
	Manager Manager
}

func NewHandler(manager Manager) *Handler{
	return &Handler{manager}
}

func (h *Handler) ScaleCluster(context.Context, *grpc_provisioner_go.ScaleClusterRequest) (*grpc_provisioner_go.ScaleClusterResponse, error) {
	panic("implement me")
}

func (h *Handler) CheckProgress(context.Context, *grpc_common_go.RequestId) (*grpc_provisioner_go.ScaleClusterResponse, error) {
	panic("implement me")
}

func (h *Handler) RemoveScale(context.Context, *grpc_common_go.RequestId) (*grpc_common_go.Success, error) {
	panic("implement me")
}


