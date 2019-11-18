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

package scaler

import (
	"github.com/nalej/grpc-common-go"
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

// ScaleCluster triggers the rescaling of a given cluster by adding or removing nodes.
func (h *Handler) ScaleCluster(_ context.Context, request *grpc_provisioner_go.ScaleClusterRequest) (*grpc_provisioner_go.ScaleClusterResponse, error) {
	err := entities.ValidScaleClusterRequest(request)
	if err != nil {
		log.Warn().Str("trace", err.DebugReport()).Msg(err.Error())
		return nil, conversions.ToGRPCError(err)
	}
	log.Debug().Interface("request", request).Msg("scale cluster")
	return h.Manager.ScaleCluster(request)
}

// CheckProgress gets an updated state of a scale request.
func (h *Handler) CheckProgress(_ context.Context, requestID *grpc_common_go.RequestId) (*grpc_provisioner_go.ScaleClusterResponse, error) {
	return h.Manager.CheckProgress(requestID)
}

// RemoveScale cancels an ongoing scale process or removes the information of an already processed one.
func (h *Handler) RemoveScale(_ context.Context, requestID *grpc_common_go.RequestId) (*grpc_common_go.Success, error) {
	err := h.Manager.RemoveScale(requestID)
	if err != nil {
		return nil, err
	}
	return &grpc_common_go.Success{}, nil
}
