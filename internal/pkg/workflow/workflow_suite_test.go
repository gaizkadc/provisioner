/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package workflow

import (
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"testing"
)

func TestWorkflowPackage(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Applications package suite")
}