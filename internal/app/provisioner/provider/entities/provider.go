/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
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
	// Decomission a cluster creates a InfrastructureOperation to decomission a cluster.
	Decomission() (entities.InfrastructureOperation, derrors.Error)
	// Scale a cluster creates a InfrastructureOperation to scale a cluster.
	Scale(request entities.ScaleRequest) (entities.InfrastructureOperation, derrors.Error)
}

