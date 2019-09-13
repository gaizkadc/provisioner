/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package decomissioner

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


func (h *Handler) DecomissionCluster(context.Context, *grpc_provisioner_go.DecomissionClusterRequest) (*grpc_provisioner_go.DecomissionClusterResponse, error) {
	panic("implement me")
}

func (h *Handler) CheckProgress(context.Context, *grpc_common_go.RequestId) (*grpc_provisioner_go.DecomissionClusterResponse, error) {
	panic("implement me")
}

func (h *Handler) RemoveDecomission(context.Context, *grpc_common_go.RequestId) (*grpc_common_go.Success, error) {
	panic("implement me")
}

