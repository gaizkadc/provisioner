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

package provisioner_cli

import (
	"fmt"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"path/filepath"
	"time"
)

type CLICommon struct {
	lastLogEntry         int
	kubeConfigOutputPath string
}

func (cc * CLICommon) printOperationLog(logPool []string) {
	if len(logPool) > cc.lastLogEntry {
		for ; cc.lastLogEntry < len(logPool); cc.lastLogEntry++ {
			log.Info().Msg(logPool[cc.lastLogEntry])
		}
	}
}

// PrintJSONResult prints the result of a provisioner operation as JSON
func (cc * CLICommon) printJSONResult(clusterName string, result entities.OperationResult) {
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
			logger.Info().Str("kubeconfig", cc.writeKubeConfig(clusterName, result.ProvisionResult.RawKubeConfig)).
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

// writeKubeConfig creates a YAML file with the resulting KubeConfig.
func (cc * CLICommon) writeKubeConfig(clusterName string, rawKubeConfig string) string {
	fileName := fmt.Sprintf("%s.yaml", clusterName)
	filePath := filepath.Join(cc.kubeConfigOutputPath, fileName)
	err := ioutil.WriteFile(filePath, []byte(rawKubeConfig), 0600)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot write kubeConfig")
	}
	return filePath
}