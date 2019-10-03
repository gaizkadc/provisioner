package commands

import (
	"github.com/golang/protobuf/jsonpb"
	"github.com/nalej/derrors"
	"github.com/nalej/grpc-installer-go"
	"github.com/nalej/grpc-provisioner-go"
	"github.com/rs/zerolog/log"
	"os"
	"strings"
)

var targetPlatform string

// GetTargetPlatform obtains the target platform
func GetTargetPlatform(platform string) (grpc_installer_go.Platform, derrors.Error) {
	switch strings.ToLower(platform) {
	case "azure":
		return grpc_installer_go.Platform_AZURE, nil
	case "baremetal":
		return grpc_installer_go.Platform_BAREMETAL, nil
	default:
		// returning a value due to the constant pointer problem
		return grpc_installer_go.Platform_MINIKUBE, derrors.NewInvalidArgumentError("unsupported platform type")
	}
}

// LoadAzureCredentials loads the content of a file into the grpc structure.
func LoadAzureCredentials(credentialsPath string) (*grpc_provisioner_go.AzureCredentials, derrors.Error) {
	credentials := &grpc_provisioner_go.AzureCredentials{}
	file, err := os.Open(credentialsPath)
	if err != nil {
		return nil, derrors.AsError(err, "cannot open credentials path")
	}
	// The unmarshalling using jsonpb is available due to the fact that the naming of the JSON fields produced
	// by Azure matches the ones defined in the protobuf json mapping.
	err = jsonpb.Unmarshal(file, credentials)
	if err != nil {
		return nil, derrors.AsError(err, "cannot unmarshal content")
	}
	log.Debug().Interface("tenantId", credentials.TenantId).Msg("azure credentials have been loaded")
	return credentials, nil
}

// ExitOnError will produce a fatal error with associated error information if an error happens.
func ExitOnError(err derrors.Error, msg string) {
	if err != nil {
		if debugLevel {
			log.Fatal().Str("trace", err.DebugReport()).Msg(msg)
		} else {
			log.Fatal().Str("err", err.Error()).Msg(msg)
		}
	}
}
