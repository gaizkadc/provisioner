/*
 * Copyright (C) 2018 Nalej - All Rights Reserved
 */

package main

import (
	"github.com/nalej/golang-template/cmd/other-app/commands"
	"github.com/nalej/golang-template/version"
)

var MainVersion string

var MainCommit string

func main() {
	version.AppVersion = MainVersion
	version.Commit = MainCommit
	commands.Execute()
}
