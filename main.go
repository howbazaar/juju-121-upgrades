// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"fmt"
	"os"

	"github.com/juju/cmd"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/names"
	"launchpad.net/gnuflag"

	"github.com/juju/juju/agent"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/mongo"
	"github.com/juju/juju/state"
)

var logger = loggo.GetLogger("juju")

func main() {
	ctx, err := cmd.DefaultContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	os.Exit(cmd.Main(&FixitCommand{}, ctx, os.Args[1:]))
}

type FixitCommand struct {
	cmd.CommandBase

	dataDir    string
	machineTag names.MachineTag

	unit            names.UnitTag
	commands        string
	showHelp        bool
	noContext       bool
	forceRemoteUnit bool
	relationId      string
	remoteUnitName  string
}

func (c *FixitCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "juju-121-upgrades",
		Args:    "<machie-id>",
		Purpose: "run the missing upgrade steps",
	}
}

func (c *FixitCommand) SetFlags(f *gnuflag.FlagSet) {
	f.StringVar(&c.dataDir, "data-dir", "/var/lib/juju", "directory for juju data")
}

func (c *FixitCommand) Init(args []string) error {
	if len(args) == 0 {
		return errors.New("missing machine-id")
	}
	var machineId string
	machineId, args = args[0], args[1:]

	if !names.IsValidMachine(machineId) {
		return errors.Errorf("%q is not a valid machine id", machineId)
	}
	c.machineTag = names.NewMachineTag(machineId)
	return cmd.CheckEmpty(args)
}

func (c *FixitCommand) Run(ctx *cmd.Context) error {

	loggo.GetLogger("juju").SetLogLevel(loggo.DEBUG)
	conf, err := agent.ReadConfig(agent.ConfigPath(c.dataDir, c.machineTag))
	if err != nil {
		return err
	}

	info, ok := conf.MongoInfo()
	if !ok {
		return errors.Errorf("no state info available")
	}
	st, err := state.Open(conf.Environment(), info, mongo.DefaultDialOpts(), environs.NewStatePolicy())
	if err != nil {
		return err
	}
	defer st.Close()

	ctx.Infof("\nStep 1: migrate individual unit ports to openedPorts collection")

	if err := state.MigrateUnitPortsToOpenedPorts(st); err != nil {
		return err
	}

	ctx.Infof("\nStep 2: create entries in meter status collection for existing units")

	if err := state.CreateUnitMeterStatus(st); err != nil {
		return err
	}

	ctx.Infof("\nStep 3: migrate machine jobs into ones with JobManageNetworking based on rules")

	if err := state.MigrateJobManageNetworking(st); err != nil {
		return err
	}

	return nil
}
