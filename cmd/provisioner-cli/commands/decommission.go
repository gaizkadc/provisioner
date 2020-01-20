/*
 * Copyright 2020 Nalej
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

package commands

import (
	"fmt"
	"github.com/nalej/grpc-infrastructure-go"
	"github.com/nalej/grpc-installer-go"
	"github.com/nalej/grpc-provisioner-go"
	"github.com/nalej/provisioner/internal/app/provisioner-cli"
	"github.com/rs/zerolog/log"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/cobra"
)

// DecommissionRequest contains the elements that will be requested to perform a decommission operation.
var decommissionRequest grpc_provisioner_go.DecommissionClusterRequest

// provisionCmd with the command to provision a new cluster.
var decommissionCmd = &cobra.Command{
	Use:   "decommission",
	Short: "Decommission a cluster",
	Long:  `Decommission an existing cluster using a specific infrastructure provider`,
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		ConfigureDecommission()
		TriggerDecommission()
	},
}

// ConfigureProvisioning configures the options using the standard gRPC structures for the provisioning command.
func ConfigureDecommission() {
	decommissionRequest.RequestId = fmt.Sprintf("cli-decommission-%s", uuid.NewV4().String())
	decommissionRequest.OrganizationId = "nalej"
	// From the CLI only management clusters may be decommissioned.
	decommissionRequest.IsManagementCluster = true
	// Only kubernetes clusters for now
	decommissionRequest.ClusterType = grpc_infrastructure_go.ClusterType_KUBERNETES
	// Determine target platform
	targetPlatform, err := GetTargetPlatform(targetPlatform)
	ExitOnError(err, "cannot determine target platform")
	decommissionRequest.TargetPlatform = targetPlatform

	// Load credentials depending on the target platform
	if decommissionRequest.TargetPlatform == grpc_installer_go.Platform_AZURE {
		if azureCredentialsPath == "" {
			log.Fatal().Msg("azureCredentialsPath must be specified")
		}
		credentials, err := LoadAzureCredentials(azureCredentialsPath)
		ExitOnError(err, "cannot load infrastructure provider credentials")
		decommissionRequest.AzureCredentials = credentials
		if azureOptions.ResourceGroup == "" {
			log.Fatal().Msg("resourceGroup must be specified")
		}
		decommissionRequest.AzureOptions = &azureOptions
	}
	cfg.LaunchService = false
}

// TriggerDecommission triggers the creation of the CLI Decommissioner and proceeds to execute the operation.
func TriggerDecommission() {
	cliDecommissioner := provisioner_cli.NewCLIDecommissioner(&decommissionRequest, cfg)
	err := cliDecommissioner.Run()
	ExitOnError(err, "decommission failed")
}

func init() {
	decommissionCmd.Flags().StringVar(&decommissionRequest.ClusterId, "name", "",
		"Name of the cluster for management cluster requests")
	decommissionCmd.Flags().StringVar(&targetPlatform, "platform", "",
		"Target plaftorm determining the provider: AZURE or BAREMETAL")
	decommissionCmd.Flags().StringVar(&azureCredentialsPath, "azureCredentialsPath", "",
		"Path to the file containing the azure credentials")
	decommissionCmd.Flags().StringVar(&azureOptions.ResourceGroup, "resourceGroup", "",
		"Target resource group where the cluster will be created. Only for Azure platform.")

	rootCmd.AddCommand(decommissionCmd)
}
