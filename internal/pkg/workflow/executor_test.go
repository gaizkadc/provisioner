/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package workflow

import (
	"github.com/nalej/derrors"
	"github.com/nalej/provisioner/internal/pkg/entities"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/rs/zerolog/log"
	"github.com/satori/go.uuid"
	"time"
)

type TestOperation struct{
	requestID string
	progress entities.TaskProgress
	started int64
}

func NewTestOperation(requestID string)entities.InfrastructureOperation{
	return &TestOperation{
		requestID:requestID,
		progress:entities.Init,
	}
}

func (to *TestOperation) RequestId() string {
	return to.requestID
}

func (to *TestOperation) Metadata() entities.OperationMetadata {
	panic("implement me")
}

func (to *TestOperation) Log() []string {
	return []string{}
}

func (to *TestOperation) Progress() entities.TaskProgress {
	return to.progress
}

func (to *TestOperation) Execute(callback func(requestId string)) derrors.Error {
	to.started = time.Now().Unix()
	log.Debug().Msg("executing test operation")
	time.Sleep(time.Second)
	to.progress = entities.Finished
	callback(to.requestID)
	return nil
}

func (to *TestOperation) Cancel() derrors.Error {
	panic("implement me")
}

func (to *TestOperation) SetProgress(progress entities.TaskProgress) {
	to.progress = progress
}

func (to *TestOperation) Result() entities.OperationResult {
	return entities.OperationResult{
		RequestId:       to.requestID,
		Type:            entities.Provision,
		Progress:        to.progress,
		ElapsedTime:     time.Now().Sub(time.Unix(to.started, 0)).Nanoseconds(),
		ErrorMsg:        "",
		ProvisionResult: nil,
	}
}



var _ = ginkgo.Describe("Executor basic tests", func(){
	executor := GetExecutor()
	ginkgo.It("should be able to execute a simple operation", func(){
	    test := NewTestOperation(uuid.NewV4().String())
	    executor.ScheduleOperation(test)
	    retries := 0
	    maxWait := 5
		for ; executor.IsManaged(test.RequestId()) && retries < maxWait; retries++ {
			time.Sleep(time.Second)
		}
		gomega.Expect(retries).ShouldNot(gomega.Equal(maxWait))
		gomega.Expect(executor.IsManaged(test.RequestId())).To(gomega.BeFalse())
		gomega.Expect(test.Progress()).To(gomega.Equal(entities.Finished))
	})

	ginkgo.It("should be able to execute queued operations", func(){
	    numOperations := 10
	    operations := make([]entities.InfrastructureOperation, 0)
	    for index := 0; index < numOperations; index ++{
	    	operations = append(operations, NewTestOperation(uuid.NewV4().String()))
		}
	    for index := 0; index < numOperations; index ++{
	    	executor.ScheduleOperation(operations[index])
		}
	    maxWait := 5
	    for index := 0; index < numOperations; index++ {
	    	retries := 0
			for ; executor.IsManaged(operations[index].RequestId()) && retries < maxWait; retries++ {
				time.Sleep(time.Second)
			}
			gomega.Expect(retries).ShouldNot(gomega.Equal(maxWait))
			gomega.Expect(executor.IsManaged(operations[index].RequestId())).To(gomega.BeFalse())
			gomega.Expect(operations[index].Progress()).To(gomega.Equal(entities.Finished))
		}
	})
})
