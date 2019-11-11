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

package provisioner_cli

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/nalej/derrors"
	grpc_provisioner_go "github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/provisioner/internal/app/provisioner/provider"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/nalej/provisioner/internal/pkg/workflow"
	"github.com/rs/zerolog/log"
)

// CLIProvisioner structure to watch the provisioning process.
type CLIProvisioner struct {
	request              *grpc_provisioner_go.ProvisionClusterRequest
	Executor             workflow.Executor
	lastLogEntry         int
	kubeConfigOutputPath string
	config               *config.Config
}

// NewCLIProvisioner creates a new CLI managed provisioner without a service.
func NewCLIProvisioner(
	request *grpc_provisioner_go.ProvisionClusterRequest,
	kubeConfigOutputPath string,
	config *config.Config) *CLIProvisioner {
	return &CLIProvisioner{
		request:              request,
		Executor:             workflow.GetExecutor(),
		lastLogEntry:         0,
		kubeConfigOutputPath: kubeConfigOutputPath,
		config:               config,
	}
}

// Run triggers the provisioning of a cluster.
func (cp *CLIProvisioner) Run() derrors.Error {
	vErr := cp.config.Validate()
	if vErr != nil {
		log.Fatal().Str("err", vErr.DebugReport()).Msg("invalid configuration")
	}
	cp.config.Print()
	log.Debug().Str("target_platform", cp.request.TargetPlatform.String()).Bool("isProduction", cp.request.IsProduction).Msg("Provision request received")
	infraProvider, err := provider.NewInfrastructureProvider(cp.request.TargetPlatform, cp.request.AzureCredentials, cp.config)
	if err != nil {
		log.Error().Msg("cannot obtain infrastructure provider")
		return err
	}
	operation, err := infraProvider.Provision(entities.NewProvisionRequest(cp.request))
	if err != nil {
		log.Error().Str("trace", err.DebugReport()).Msg("cannot create provision operation")
		return err
	}
	cp.Executor.ScheduleOperation(operation)
	start := time.Now()
	checks := 0
	for cp.Executor.IsManaged(cp.request.RequestId) {
		time.Sleep(15 * time.Second)
		cp.printOperationLog(operation.Log())
		if checks%4 == 0 {
			fmt.Printf("Provision operation %s - %s\n", entities.TaskProgressToString[operation.Progress()], time.Since(start).String())
		}
		checks++
	}
	elapsed := time.Since(start)
	fmt.Println("Provisioning took ", elapsed)
	// Process the result
	result := operation.Result()
	cp.printJSONResult(result)
	// cp.printTableResult(result)
	return nil
}

// printOperationLog prints the logs entries to stdout as they become available.
func (cp *CLIProvisioner) printOperationLog(logPool []string) {
	if len(logPool) > cp.lastLogEntry {
		for ; cp.lastLogEntry < len(logPool); cp.lastLogEntry++ {
			log.Info().Msg(logPool[cp.lastLogEntry])
		}
	}
}

func (cp *CLIProvisioner) printJSONResult(result entities.OperationResult) {
	logger := log.With().
		Str("request_id", result.RequestId).
		Str("type", entities.ToOperationTypeString[result.Type]).
		Str("progress", entities.TaskProgressToString[result.Progress]).
		Str("elapsed_time", time.Duration(result.ElapsedTime).String()).
		Logger()

	if result.Progress == entities.Error {
		logger.Error().Msg(result.ErrorMsg)
	} else {
		if result.ProvisionResult != nil {
			logger.Info().Str("kubeconfig", cp.writeKubeConfig(*result.ProvisionResult)).
				Str("ingress_ip", result.ProvisionResult.StaticIPAddresses.Ingress).
				Str("dns_ip", result.ProvisionResult.StaticIPAddresses.DNS).
				Str("coredns_ip", result.ProvisionResult.StaticIPAddresses.CoreDNSExt).
				Str("vpnserver_ip", result.ProvisionResult.StaticIPAddresses.VPNServer).
				Msg("Finished provision operation")
		} else {
			logger.Warn().Msg("Expecting provisioning result")
		}
	}
}

// printResult prints the result of the command.
func (cp *CLIProvisioner) printTableResult(result entities.OperationResult) {
	writer := NewTabWriterHelper()
	writer.Println("Request:\t", result.RequestId)
	writer.Println("Type:\t", entities.ToOperationTypeString[result.Type])
	writer.Println("Progress:\t", entities.TaskProgressToString[result.Progress])
	writer.Println("Elapsed Time:\t", time.Duration(result.ElapsedTime).String())
	if result.Progress == entities.Error {
		writer.Println("Error:\t", result.ErrorMsg)
	} else {
		if result.ProvisionResult != nil {
			writer.Println("KubeConfig:\t", cp.writeKubeConfig(*result.ProvisionResult))
			writer.Println("Ingress IP:\t", result.ProvisionResult.StaticIPAddresses.Ingress)
			writer.Println("DNS IP:\t", result.ProvisionResult.StaticIPAddresses.DNS)
			writer.Println("CoreDNS IP:\t", result.ProvisionResult.StaticIPAddresses.CoreDNSExt)
			writer.Println("VPN Server IP:\t", result.ProvisionResult.StaticIPAddresses.VPNServer)
		} else {
			log.Warn().Msg("expecting provisioning result")
		}
	}
	err := writer.Flush()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot write result to stdout")
	}
}

// writeKubeConfig creates a YAML file with the resulting KubeConfig.
func (cp *CLIProvisioner) writeKubeConfig(result entities.ProvisionResult) string {
	fileName := fmt.Sprintf("%s.yaml", cp.request.ClusterName)
	filePath := filepath.Join(cp.kubeConfigOutputPath, fileName)
	err := ioutil.WriteFile(filePath, []byte(result.RawKubeConfig), 0600)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot write kubeConfig")
	}
	return filePath
}
