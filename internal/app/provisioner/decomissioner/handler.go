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

func NewHandler(manager Manager) *Handler {
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
