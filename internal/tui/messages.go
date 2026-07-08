package tui

import "ctrwatch/internal/runtime"

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
	Containers []runtime.Container
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

type serverConnectMsg struct {
	serverIdx  int
	client     *runtime.Client
	containers []string
	cleanup    func()
	err        error
}

type visibleLogLine struct {
	text   string
	stderr bool
}
