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
	grpc_infrastructure_go "github.com/nalej/grpc-infrastructure-go"
	grpc_installer_go "github.com/nalej/grpc-installer-go"
	grpc_provisioner_go "github.com/nalej/grpc-provisioner-go"
	provisioner_cli "github.com/nalej/provisioner/internal/app/provisioner-cli"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/rs/zerolog/log"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/cobra"
)

var clusterRequest grpc_provisioner_go.ClusterRequest

var appCluster bool

// mngtCmd with the base command for management operations.
var mngtCmd = &cobra.Command{
	Use:     "management",
	Aliases: []string{"mngt"},
	Short:   "Management related operations on a cluster",
	Long:    `Management related operations on a cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		_ = cmd.Help()
	},
}

var getKubeConfigLongHelp = `
Retrieve the Kubeconfig to access a given Kubernetes cluster.

This command will use the underlying infrastructure provider to retrieve
the kubeconfig file that gives access to the kubernetes cluster. Notice that
depending on the provider, access to the control/master nodes may not be possible
so using the API of the provider is required.
`

var getKubeConfigExample = `

# Obtain the kubeconfig of an application cluster deployed in AZURE
provisioner-cli management kubeconfig <clusterID> --azureCredentialsPath <full_credentials_path> --platform AZURE --resourceGroup dev --appCluster

`

// getKubeConfigCmd with the command to retrieve the kube config of a given cluster.
var getKubeConfigCmd = &cobra.Command{
	Use:     "kubeconfig <clusterID>",
	Aliases: []string{"config"},
	Short:   "Retrieve the kubeconfig of a cluster",
	Long:    getKubeConfigLongHelp,
	Example: getKubeConfigExample,
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		// Notice that clusterID matches clusterName for management cluster.
		ConfigureClusterRequest(args[0])
		TriggerGetKubeConfig()
	},
}

func ConfigureClusterRequest(clusterID string) {
	clusterRequest.RequestId = fmt.Sprintf("cli-mngt-%s", uuid.NewV4().String())
	clusterRequest.OrganizationId = "nalej"
	clusterRequest.ClusterId = clusterID
	clusterRequest.ClusterType = grpc_infrastructure_go.ClusterType_KUBERNETES
	clusterRequest.IsManagementCluster = !appCluster
	// Determine target platform
	targetPlatform, err := GetTargetPlatform(targetPlatform)
	ExitOnError(err, "cannot determine target platform")
	clusterRequest.TargetPlatform = targetPlatform
	// Load credentials depending on the target platform
	if clusterRequest.TargetPlatform == grpc_installer_go.Platform_AZURE {
		if azureCredentialsPath == "" {
			log.Fatal().Msg("azureCredentialsPath must be specified")
		}
		credentials, err := LoadAzureCredentials(azureCredentialsPath)
		ExitOnError(err, "cannot load infrastructure provider credentials")
		clusterRequest.AzureCredentials = credentials
		if azureOptions.ResourceGroup == "" {
			log.Fatal().Msg("resourceGroup must be specified")
		}
		clusterRequest.AzureOptions = &azureOptions
	}
	cfg.LaunchService = false

}

// TriggerGetKubeConfig triggers the creation of the CLI management helper and proceeds to execute the operation.
func TriggerGetKubeConfig() {
	cliManager := provisioner_cli.NewCLIManagement(&clusterRequest, entities.GetKubeConfig, kubeConfigOutputPath, cfg)
	err := cliManager.Run()
	ExitOnError(err, "get kubeconfig failed")
}

func init() {
	getKubeConfigCmd.Flags().StringVar(&targetPlatform, "platform", "",
		"Target plaftorm determining the provider: AZURE or BAREMETAL")
	getKubeConfigCmd.Flags().StringVar(&azureOptions.ResourceGroup, "resourceGroup", "",
		"Target resource group where the cluster will be created. Only for Azure platform.")
	getKubeConfigCmd.Flags().StringVar(&azureCredentialsPath, "azureCredentialsPath", "",
		"Path to the file containing the azure credentials")
	getKubeConfigCmd.Flags().StringVar(&kubeConfigOutputPath, "kubeConfigOutputPath", "/tmp/",
		"Path to directory where the resulting kubeconfig will be stored")
	getKubeConfigCmd.Flags().BoolVar(&appCluster, "appCluster", false,
		"Set to true if the target cluster is an application cluster.")
	mngtCmd.AddCommand(getKubeConfigCmd)
	rootCmd.AddCommand(mngtCmd)
}
