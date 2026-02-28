package conntrack

type Logger interface {
	Trace1(format string, arg1 any)
	Trace2(format string, arg1, arg2 any)
	Trace3(format string, arg1, arg2, arg3 any)
	Trace4(format string, arg1, arg2, arg3, arg4 any)
	Trace5(format string, arg1, arg2, arg3, arg4, arg5 any)
}
