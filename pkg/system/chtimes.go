package system

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

// maxTime returns the maximum time that can be set by os.Chtimes on this platform
// 64 bit Timespec supports up to 2262 and 32 bit Timespec supports up to 2038
func maxTime() time.Time {
	if unsafe.Sizeof(syscall.Timespec{}.Nsec) == 8 {
		// This is a 64 bit timespec
		// os.Chtimes limits time to the following
		return time.Unix(0, 1<<63-1)
	}

	// This is a 32 bit timespec
	return time.Unix(1<<31-1, 0)
}

// Chtimes changes the access time and modified time of a file at the given path
func Chtimes(name string, atime time.Time, mtime time.Time) error {
	unixMinTime := time.Unix(0, 0)
	unixMaxTime := maxTime()

	// If the modified time is prior to the Unix Epoch, or after the
	// end of Unix Time, os.Chtimes has undefined behavior
	// default to Unix Epoch in this case, just in case

	if atime.Before(unixMinTime) || atime.After(unixMaxTime) {
		atime = unixMinTime
	}

	if mtime.Before(unixMinTime) || mtime.After(unixMaxTime) {
		mtime = unixMinTime
	}

	if err := os.Chtimes(name, atime, mtime); err != nil {
		return err
	}

	return nil
}
