/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package entities

import (
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-installer-go"
	"github.com/nalej/grpc-provisioner-go"
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

type OperationType int

const (
	Provision OperationType = iota
	Decomission
	Scale
)

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
	ElapsedTime string
	// ErrorMsg contains a description of the error in case the operation failed.
	ErrorMsg string
	// ProvisionResult with the results of a provisioning operation.
	ProvisionResult *ProvisionResult
}

func (or *OperationResult) ToProvisionClusterResult() (*grpc_provisioner_go.ProvisionClusterResponse, derrors.Error) {
	if or.Type != Provision {
		return nil, derrors.NewInternalError("cannot create provision cluster response for other type").WithParams(or)
	}

	kubeConfig := ""
	var staticIpAddresses *grpc_installer_go.StaticIPAddresses

	if or.ProvisionResult != nil {
		kubeConfig = or.ProvisionResult.RawKubeConfig
		// TODO Add resulting ip addresses
		staticIpAddresses = &grpc_installer_go.StaticIPAddresses{
			UseStaticIp: true,
			Ingress:     "",
			Dns:         "",
			ZtPlanet:    "",
			CorednsExt:  "",
			VpnServer:   "",
		}
	}

	return &grpc_provisioner_go.ProvisionClusterResponse{
		RequestId: or.RequestId,
		State:     ToGRPCProvisionProgress[or.Progress],
		// TODO Change proto type
		ElapsedTime:       0,
		Error:             or.ErrorMsg,
		RawKubeConfig:     kubeConfig,
		StaticIpAddresses: staticIpAddresses,
	}, nil
}
