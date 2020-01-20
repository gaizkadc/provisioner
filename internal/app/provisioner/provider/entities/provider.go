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

package entities

import (
	"github.com/nalej/derrors"
	"github.com/nalej/provisioner/internal/pkg/entities"
)

// InfrastructureProvider interface defining the operations a cloud or baremetal infrastructure
// provider must support in order to be used by the platform. This interface serves as the
// abstraction layer between the requests send by the user and their global execution, and the
// specifics of how each operation will be performed depending on the type of provider.
type InfrastructureProvider interface {
	// Provision a cluster creates a InfrastructureOperation to provision a new cluster.
	Provision(request entities.ProvisionRequest) (entities.InfrastructureOperation, derrors.Error)
	// Decommission a cluster creates a InfrastructureOperation to decommission a cluster.
	Decommission(request entities.DecommissionRequest) (entities.InfrastructureOperation, derrors.Error)
	// Scale a cluster creates a InfrastructureOperation to scale a cluster.
	Scale(request entities.ScaleRequest) (entities.InfrastructureOperation, derrors.Error)
	// GetKubeConfig retrieves the KubeConfig file to access the management layer of Kubernetes.
	GetKubeConfig(request entities.ClusterRequest) (entities.InfrastructureOperation, derrors.Error)
}
