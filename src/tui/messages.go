package tui

import (
	"ctrwatch/src/runtime"
	"ctrwatch/src/ssh"
)

type viewType int

const (
	viewLogs viewType = iota
	viewPS
	viewStats
	viewServers
)

type logBatchMsg []runtime.LogLine

type statsMsg struct {
	Stats map[string]*runtime.ContainerStats
}

type containersListMsg struct {
	Containers []containerListItem
	Err        error
}

type inspectMsg struct {
	Inspect *runtime.ContainerInspect
	Err     error
}

type diffMsg struct {
	Changes []runtime.Change
	Err     error
}

type topMsg struct {
	Top *runtime.TopResponse
	Err error
}

type serverStateTickMsg struct{}

type serverConnectMsg struct {
	serverIdx int
	endpoints []ssh.ResolvedServer
	err       error
}

type visibleLogLine struct {
	text   string
	stderr bool
}

type containerListItem struct {
	Client    *runtime.Client
	Container runtime.Container
}
