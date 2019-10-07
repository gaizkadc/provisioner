/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package decomissioner

import (
	"github.com/nalej/provisioner/internal/pkg/config"
	"sync"
)

type Manager struct {
	sync.Mutex
	Config config.Config
}

func NewManager(config config.Config) Manager {
	return Manager{
		Config: config,
	}
}
