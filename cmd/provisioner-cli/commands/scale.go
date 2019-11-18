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

// ScaleRequest contains the elements that will be requested to perform a scale operation.
var scaleRequest grpc_provisioner_go.ScaleClusterRequest

// provisionCmd with the command to provision a new cluster.
var scaleCmd = &cobra.Command{
	Use:   "scale",
	Short: "Scale a cluster",
	Long:  `Scale an existing cluster using a specific infrastructure provider`,
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		ConfigureScale()
		TriggerScale()
	},
}

// ConfigureProvisioning configures the options using the standard gRPC structures for the provisioning command.
func ConfigureScale() {
	scaleRequest.RequestId = fmt.Sprintf("cli-scale-%s", uuid.NewV4().String())
	scaleRequest.OrganizationId = "nalej"
	// From the CLI only management clusters may be scaled.
	scaleRequest.IsManagementCluster = true
	// Only kubernetes clusters for now
	scaleRequest.ClusterType = grpc_infrastructure_go.ClusterType_KUBERNETES
	// Determine target platform
	targetPlatform, err := GetTargetPlatform(targetPlatform)
	ExitOnError(err, "cannot determine target platform")
	scaleRequest.TargetPlatform = targetPlatform

	// Load credentials depending on the target platform
	if scaleRequest.TargetPlatform == grpc_installer_go.Platform_AZURE {
		if azureCredentialsPath == "" {
			log.Fatal().Msg("azureCredentialsPath must be specified")
		}
		credentials, err := LoadAzureCredentials(azureCredentialsPath)
		ExitOnError(err, "cannot load infrastructure provider credentials")
		scaleRequest.AzureCredentials = credentials
		if azureOptions.ResourceGroup == "" {
			log.Fatal().Msg("resourceGroup must be specified")
		}
		scaleRequest.AzureOptions = &azureOptions
	}
	cfg.LaunchService = false
}

// TriggerScale triggers the creation of the CLI Scaler and proceeds to execute the operation.
func TriggerScale() {
	cliScaler := provisioner_cli.NewCLIScaler(&scaleRequest, cfg)
	err := cliScaler.Run()
	ExitOnError(err, "scaling failed")
}

func init() {
	scaleCmd.Flags().StringVar(&scaleRequest.ClusterId, "name", "",
		"Name of the cluster for management cluster requests")
	scaleCmd.Flags().StringVar(&scaleRequest.ClusterId, "clusterID", "",
		"Cluster ID for application cluster requests")
	scaleCmd.Flags().Int64Var(&scaleRequest.NumNodes, "numNodes", 3,
		"Number of nodes to scale the cluster")
	scaleCmd.Flags().StringVar(&targetPlatform, "platform", "",
		"Target plaftorm determining the provider: AZURE or BAREMETAL")
	scaleCmd.Flags().StringVar(&azureCredentialsPath, "azureCredentialsPath", "",
		"Path to the file containing the azure credentials")
	scaleCmd.Flags().StringVar(&azureOptions.ResourceGroup, "resourceGroup", "",
		"Target resource group where the cluster will be created. Only for Azure platform.")

	rootCmd.AddCommand(scaleCmd)
}
