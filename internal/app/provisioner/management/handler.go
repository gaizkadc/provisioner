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

package management

import (
	"github.com/nalej/grpc-provisioner-go"
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

// GetKubeConfig retrieves the KubeConfig file to access the management layer of Kubernetes.
func (h *Handler) GetKubeConfig(_ context.Context, request *grpc_provisioner_go.ClusterRequest) (*grpc_provisioner_go.KubeConfigResponse, error) {
	err := entities.ValidClusterRequest(request)
	if err != nil {
		log.Warn().Str("trace", err.DebugReport()).Msg(err.Error())
		return nil, conversions.ToGRPCError(err)
	}
	return h.Manager.GetKubeConfig(request)
}
