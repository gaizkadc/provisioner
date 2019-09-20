/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package entities

import "github.com/nalej/derrors"

// InfrastructureOperation represents an ongoing operation being performed by the infrastructure
// provider.
type InfrastructureOperation interface {
	// RequestID returns the request identifier associated with this operation
	RequestID() string
	// Metadata returns the operation associated metadata
	Metadata() OperationMetadata
	// Log returns the information associated with the execution of the operation
	Log() []string
	// Progress returns the operation state
	Progress() TaskProgress
	// Execute triggers the execution of the operation. The callback function on the execute is expected to be
	// called when the operation finish its execution independently of the status.
	Execute(func(requestID string))
	// Cancel triggers the cancellation of the operation
	Cancel() derrors.Error
	// SetProgress set a new progress to the ongoing operation.
	SetProgress(progress TaskProgress)
	// Result returns the operation result if this operation is successful
	Result() OperationResult
}

// OperationMetadata associated with the operation.
type OperationMetadata struct {
	// OrganizationID associated with the operation.
	OrganizationID string
	// ClusterID target of the operation.
	ClusterID string
	// RequestID for tracking purposes.
	RequestID string
}
