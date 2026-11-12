package fakebw

// CallCount returns how many calls the log holds, so a run can mark where its
// own calls begin. A log that does not exist yet holds none.
func CallCount(logPath string) (int, error) {
	calls, err := ReadLog(logPath)
	if err != nil {
		return 0, err
	}
	return len(calls), nil
}

// CallsSince returns the bw arguments of each call the log holds from index
// start on, in order. An index past the end yields no calls.
func CallsSince(logPath string, start int) ([][]string, error) {
	calls, err := ReadLog(logPath)
	if err != nil {
		return nil, err
	}
	if start > len(calls) {
		start = len(calls)
	}
	var argv [][]string
	for _, call := range calls[start:] {
		argv = append(argv, call.Argv)
	}
	return argv, nil
}
