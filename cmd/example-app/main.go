/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
 */

// This is an example of an executable command.

package main

import (
	"github.com/nalej/golang-template/cmd/example-app/commands"
	"github.com/nalej/golang-template/version"
)

var MainVersion string

var MainCommit string

func main() {
	version.AppVersion = MainVersion
	version.Commit = MainCommit
	commands.Execute()
}
