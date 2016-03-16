package libcontainerd

import (
	"strings"
	"syscall"
)

// setupEnvironmentVariables convert a string array of environment variables
// into a map as required by the HCS. Source array is in format [v1=k1] [v2=k2] etc.
func setupEnvironmentVariables(a []string) map[string]string {
	r := make(map[string]string)
	for _, s := range a {
		arr := strings.Split(s, "=")
		if len(arr) == 2 {
			r[arr[0]] = arr[1]
		}
	}
	return r
}

// ArgsFromSlice returns the value to place in the Process.Args field,
// given a slice of arguments (starting with the executable) and whether
// the arguments have already been escaped.
func ArgsFromSlice(args []string, escaped bool) string {
	if escaped {
		return strings.Join(args, " ")
	}
	s := ""
	for i, a := range args {
		if i != 0 {
			s += " "
		}
		s += syscall.EscapeArg(a)
	}
	return s
}
