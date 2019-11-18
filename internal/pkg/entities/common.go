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

package entities

import (
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-installer-go"
	"github.com/nalej/grpc-provisioner-go"
	"github.com/rs/zerolog/log"
)

// TaskProgress enum with the progress of a given infrastructure operation.
type TaskProgress int

const (
	Init TaskProgress = iota
	Registered
	InProgress
	Error
	Finished
)

var TaskProgressToString = map[TaskProgress]string{
	Init:       "Init",
	Registered: "Registered",
	InProgress: "InProgress",
	Error:      "Error",
	Finished:   "Finished",
}

// ToGRPCProvisionProgress contains the mapping between the internal and gRPC progress structure.
var ToGRPCProvisionProgress = map[TaskProgress]grpc_provisioner_go.ProvisionProgress{
	Init:       grpc_provisioner_go.ProvisionProgress_INIT,
	Registered: grpc_provisioner_go.ProvisionProgress_REGISTERED,
	InProgress: grpc_provisioner_go.ProvisionProgress_IN_PROGRESS,
	Error:      grpc_provisioner_go.ProvisionProgress_ERROR,
	Finished:   grpc_provisioner_go.ProvisionProgress_FINISHED,
}

// OperationType defines the base type for an enum with the types of operations supported.
type OperationType int

const (
	// Provision cluster operation.
	Provision OperationType = iota
	// Decomission cluster operation.
	Decomission
	// Scale cluster operation.
	Scale
)

// ToOperationTypeString map associating enum values with the string representation.
var ToOperationTypeString = map[OperationType]string{
	Provision:   "Provision",
	Decomission: "Decomission",
	Scale:       "Scale",
}

// OperationResult with the result of a successful infrastructure operation
type OperationResult struct {
	// RequestId with the request identifier
	RequestId string
	// Type of operation being executed
	Type OperationType
	// Progress with the state of the operation.
	Progress TaskProgress
	// ElapsedTime with the time since the operation was launched.
	ElapsedTime int64
	// ErrorMsg contains a description of the error in case the operation failed.
	ErrorMsg string
	// ProvisionResult with the results of a provisioning operation.
	ProvisionResult *ProvisionResult
}

// ToProvisionClusterResult transforms an operation result into a ProvisionClusterResponse.
func (or *OperationResult) ToProvisionClusterResult() (*grpc_provisioner_go.ProvisionClusterResponse, derrors.Error) {
	if or.Type != Provision {
		log.Error().Interface("result", or).Msg("cannot create scale cluster response for other type")
		return nil, derrors.NewInternalError("cannot create provision cluster response for other type").WithParams(or)
	}
	kubeConfig := ""
	var staticIPAddresses *grpc_installer_go.StaticIPAddresses
	if or.ProvisionResult != nil {
		kubeConfig = or.ProvisionResult.RawKubeConfig
		// TODO Add resulting ip addresses
		staticIPAddresses = &grpc_installer_go.StaticIPAddresses{
			UseStaticIp: true,
			Ingress:     or.ProvisionResult.StaticIPAddresses.Ingress,
			Dns:         or.ProvisionResult.StaticIPAddresses.DNS,
			ZtPlanet:    or.ProvisionResult.StaticIPAddresses.ZtPlanet,
			CorednsExt:  or.ProvisionResult.StaticIPAddresses.CoreDNSExt,
			VpnServer:   or.ProvisionResult.StaticIPAddresses.VPNServer,
		}
	}
	return &grpc_provisioner_go.ProvisionClusterResponse{
		ClusterName:       or.ProvisionResult.ClusterName,
		Hostname:          or.ProvisionResult.Hostname,
		RequestId:         or.RequestId,
		State:             ToGRPCProvisionProgress[or.Progress],
		ElapsedTime:       or.ElapsedTime,
		Error:             or.ErrorMsg,
		RawKubeConfig:     kubeConfig,
		StaticIpAddresses: staticIPAddresses,
	}, nil
}

// ToScaleClusterResult transforms an operation result into a ScaleClusterResponse.
func (or *OperationResult) ToScaleClusterResult() (*grpc_provisioner_go.ScaleClusterResponse, derrors.Error) {
	if or.Type != Scale {
		log.Error().Interface("result", or).Msg("cannot create scale cluster response for other type")
		return nil, derrors.NewInternalError("cannot create scale cluster response for other type").WithParams(or)
	}
	return &grpc_provisioner_go.ScaleClusterResponse{
		RequestId:   or.RequestId,
		State:       ToGRPCProvisionProgress[or.Progress],
		ElapsedTime: or.ElapsedTime,
		Error:       or.ErrorMsg,
	}, nil
}
