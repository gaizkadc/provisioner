/*
 * Copyright (C) 2019 Nalej - All Rights Reserved
 */

package main

import (
	"github.com/nalej/provisioner/cmd/provisioner/commands"
	"github.com/nalej/provisioner/version"
)

var MainVersion string

var MainCommit string

func main() {
	version.AppVersion = MainVersion
	version.Commit = MainCommit
	commands.Execute()
}
