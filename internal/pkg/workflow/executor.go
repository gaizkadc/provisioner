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

package workflow

import (
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/rs/zerolog/log"
	"sync"
)

const MaxConcurrentOperation = 5

var executorInstance Executor
var onceExecutor sync.Once

// Executor structure inspired by the one on the installer component. In this case, the executor
// is responsible of executing a set of provider operations.
type Executor struct {
	sync.Mutex
	// Queue of operation to be executed
	Queue []entities.InfrastructureOperation
	// OnExecution contains the operations being executed at the moment
	OnExecution map[string]entities.InfrastructureOperation
	// Managed map of operations.
	Managed map[string]bool
}

func NewExecutor() Executor {
	return Executor{
		Queue:       make([]entities.InfrastructureOperation, 0),
		OnExecution: make(map[string]entities.InfrastructureOperation, 0),
		Managed:     make(map[string]bool, 0),
	}
}

func GetExecutor() Executor {
	onceExecutor.Do(func() {
		executorInstance = NewExecutor()
	})
	return executorInstance
}

// ScheduleOperation schedules an operation for execution
func (e *Executor) ScheduleOperation(operation entities.InfrastructureOperation) {
	operation.SetProgress(entities.Registered)
	e.Lock()
	defer e.Unlock()
	e.Managed[operation.RequestID()] = true
	if len(e.OnExecution) > MaxConcurrentOperation {
		log.Debug().Msg("operation has been queued")
		e.Queue = append(e.Queue, operation)
	} else {
		e.OnExecution[operation.RequestID()] = operation
		go operation.Execute(e.operationCallback)
	}
}

// IsManaged enables the manager to check if the operation is queued or in progress
func (e *Executor) IsManaged(requestID string) bool {
	e.Lock()
	defer e.Unlock()
	_, exists := e.Managed[requestID]
	return exists
}

// rescheduleNextOperation checks the queued list and picks the first element and proceeds with its execution.
func (e *Executor) rescheduleNextOperation() {
	e.Lock()
	defer e.Unlock()
	log.Debug().Int("queued", len(e.Queue)).Int("onExecution", len(e.OnExecution)).Msg("rescheduling next operation")
	if len(e.Queue) == 0 {
		return
	}
	// Pick first element of the queue and schedule it.
	first := e.Queue[0]
	e.Queue = e.Queue[1:]
	e.OnExecution[first.RequestID()] = first
	go first.Execute(e.operationCallback)
}

// operationCallback function called when the operation finished its execution. This enables rescheduling the next
// operations from the queue.
func (e *Executor) operationCallback(requestID string) {
	log.Debug().Str("requestID", requestID).Msg("operation callback received")
	e.Lock()
	defer e.Unlock()
	_, exists := e.OnExecution[requestID]
	if !exists {
		log.Error().Str("requestID", requestID).Msg("attempting to remove a request id not managed by the executor")
	} else {
		delete(e.OnExecution, requestID)
		delete(e.Managed, requestID)
		go e.rescheduleNextOperation()
	}
}
