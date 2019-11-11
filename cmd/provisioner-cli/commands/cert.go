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

package commands

import (
	"fmt"
	"io/ioutil"

	"github.com/nalej/derrors"
	"github.com/nalej/provisioner/internal/app/provisioner/certmngr"
	"github.com/nalej/provisioner/internal/pkg/config"
	"github.com/spf13/cobra"
)

// Overall provisioner configuration
var certCfg = &config.Config{}

// provisionCmd with the command to provision a new cluster.
var certCmd = &cobra.Command{
	Use:   "cert",
	Short: "Certificate related operations on a provisioned cluster",
	Long:  `Certificate related operations on a provisioned cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		_ = cmd.Help()
	},
}

// checkCertStatusCmd is a command to check the status of the issued certificate.
var checkCertStatusCmd = &cobra.Command{
	Use:   "status [kubeConfigPath]",
	Short: "Check the status of the cluster certificate",
	Long:  "Check the status of the cluster certificate",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		SetupLogging()
		TriggerCheck(args[0])
	},
}

// TriggerCheck creates a cert manager to check the status of the requested certificate
func TriggerCheck(kubeConfigPath string) {
	cfg.Print()
	certManager := certmngr.NewCertManagerHelper(certCfg)
	rawKubeConfig, rerr := ioutil.ReadFile(kubeConfigPath)
	ExitOnError(derrors.AsError(rerr, "cannot read kubeconfig file"), "cannot read file")
	err := certManager.Connect(string(rawKubeConfig))
	ExitOnError(err, "Unable to connect to the target Kubernetes")
	err = certManager.CheckCertificateIssuer()
	ExitOnError(err, "Unable to check certificate status")
	fmt.Println("Certificate has been issued")
}

func init() {
	checkCertStatusCmd.Flags().StringVar(&certCfg.TempPath, "tempPath", "/tmp/",
		"Directory to store temporal files")
	certCmd.AddCommand(checkCertStatusCmd)
	rootCmd.AddCommand(certCmd)
}
