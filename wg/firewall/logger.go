package firewall

import (
	"github.com/google/uuid"
	"remoon.net/well/wg/conntrack/types"
)

type NoopFlowLogger struct{}

func (NoopFlowLogger) StoreEvent(flowEvent types.EventFields)              {}
func (NoopFlowLogger) GetEvents() []*types.Event                           { return []*types.Event{} }
func (NoopFlowLogger) DeleteEvents([]uuid.UUID)                            {}
func (NoopFlowLogger) Close()                                              {}
func (NoopFlowLogger) Enable()                                             {}
func (NoopFlowLogger) UpdateConfig(dnsCollection, exitNodeCollection bool) {}

type NoopLogger struct{}

func (NoopLogger) Trace1(format string, arg1 any)                         {}
func (NoopLogger) Trace2(format string, arg1, arg2 any)                   {}
func (NoopLogger) Trace3(format string, arg1, arg2, arg3 any)             {}
func (NoopLogger) Trace4(format string, arg1, arg2, arg3, arg4 any)       {}
func (NoopLogger) Trace5(format string, arg1, arg2, arg3, arg4, arg5 any) {}
