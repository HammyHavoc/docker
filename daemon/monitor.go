package daemon

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/runconfig"
)

// StateChanged updates daemon state changes from containerd
func (daemon *Daemon) StateChanged(id string, e libcontainerd.StateInfo) error {
	logrus.Debugln("StateChanged()", e)

	c := daemon.containers.Get(id)
	if c == nil {
		logrus.Debugln("StateChanged: Could not get container with id", id)
		return fmt.Errorf("no such container: %s", id)
	}

	switch e.State {
	case libcontainerd.StateOOM:
		logrus.Debugln("Handling StateOOM event")
		// StateOOM is Linux specific and should never be hit on Windows
		if runtime.GOOS == "windows" {
			return errors.New("Received StateOOM from libcontainerd on Windows. This should never happen.")
		}
		daemon.LogContainerEvent(c, "oom")
		logrus.Debugln("Finished handling StateOOM event")
	case libcontainerd.StateExit:
		logrus.Debugln("Handling StateExit event")
		c.Lock()
		defer c.Unlock()
		c.Wait()
		logrus.Debugln("StateExit calling reset")
		c.Reset(false)
		logrus.Debugln("StateExit calling SetStopped")
		c.SetStopped(platformConstructExitStatus(e))
		attributes := map[string]string{
			"exitCode": strconv.Itoa(int(e.ExitCode)),
		}
		daemon.LogContainerEventWithAttributes(c, "die", attributes)
		logrus.Debugln("StateExit calling daemon.Cleanup")
		daemon.Cleanup(c)
		// FIXME: here is race condition between two RUN instructions in Dockerfile
		// because they share same runconfig and change image. Must be fixed
		// in builder/builder.go
		logrus.Debugln("Finished handling StateOOM event - calling ToDisk()")
		return c.ToDisk()
	case libcontainerd.StateRestart:
		logrus.Debugln("Handling StateRestart event")
		c.Lock()
		defer c.Unlock()
		logrus.Debugln("StateRestart calling Reset")
		c.Reset(false)
		c.RestartCount++
		logrus.Debugln("StartRestart - RestartCount=", c.RestartCount)
		c.SetRestarting(platformConstructExitStatus(e))
		logrus.Debugln("StartRestart after SetRestarting, exitCode=", e.ExitCode)
		attributes := map[string]string{
			"exitCode": strconv.Itoa(int(e.ExitCode)),
		}
		daemon.LogContainerEventWithAttributes(c, "die", attributes)
		logrus.Debugln("Finished handling StateRestart event - calling ToDisk()")
		return c.ToDisk()
	case libcontainerd.StateExitProcess:
		logrus.Debugln("Handling StateExitProcess event")
		c.Lock()
		defer c.Unlock()
		if execConfig := c.ExecCommands.Get(e.ProcessID); execConfig != nil {
			logrus.Debugln("StateExitProcess have an execConfig")
			ec := int(e.ExitCode)
			execConfig.ExitCode = &ec
			execConfig.Running = false
			logrus.Debugln("StateExitProcess calling execConfig.Wait()")
			execConfig.Wait()
			logrus.Debugln("StateExitProcess calling CloseStreams")
			if err := execConfig.CloseStreams(); err != nil {
				logrus.Errorf("%s: %s", c.ID, err)
			}
			logrus.Debugln("StateExitProcess streams have been closed, calling ExecCommands.Delete on", execConfig.ID)

			// remove the exec command from the container's store only and not the
			// daemon's store so that the exec command can be inspected.
			c.ExecCommands.Delete(execConfig.ID)
		} else {
			logrus.Warnf("Ignoring StateExitProcess for %v but no exec command found", e)
		}
		logrus.Debugln("Finished handling StateExitProcess event")
	case libcontainerd.StateStart, libcontainerd.StateRestore:
		logrus.Debugln("Handling StateStart or StateRestore event")
		c.SetRunning(int(e.Pid), e.State == libcontainerd.StateStart)
		c.HasBeenManuallyStopped = false
		if err := c.ToDisk(); err != nil {
			c.Reset(false)
			return err
		}
		logrus.Debugln("Finished handling StateStart or StateRestore event")
	case libcontainerd.StatePause:
		c.Paused = true
		daemon.LogContainerEvent(c, "pause")
	case libcontainerd.StateResume:
		c.Paused = false
		daemon.LogContainerEvent(c, "unpause")
	}

	logrus.Debugln("Returning nil at bottom of StateChanged()", e)
	return nil
}

// AttachStreams is called by libcontainerd to connect the stdio.
func (daemon *Daemon) AttachStreams(id string, iop libcontainerd.IOPipe) error {
	var s *runconfig.StreamConfig
	logrus.Debugln("Monitor.go AttachStreams on", id)
	logrus.Debugln(" - stderr==nil", (iop.Stderr == nil))
	logrus.Debugln(" - stdout==nil", (iop.Stdout == nil))
	logrus.Debugln(" - stdin==nil", (iop.Stdin == nil))
	c := daemon.containers.Get(id)
	if c == nil {
		logrus.Debugln("Container not found - going down using ec.StreamConfig path")
		ec, err := daemon.getExecConfig(id)
		if err != nil {
			logrus.Debugln("Failed to getExecConfig on", id, err)
			return fmt.Errorf("no such exec/container: %s", id)
		}
		s = ec.StreamConfig
	} else {
		logrus.Debugln("Container found - going down using c.StreamConfig path")
		s = c.StreamConfig
		if err := daemon.StartLogging(c); err != nil {
			c.Reset(false)
			return err
		}
	}

	if stdin := s.Stdin(); stdin != nil {
		if iop.Stdin != nil {
			logrus.Debugln("spinning up goroutine for iop.Stdin, stdin copy")
			go func() {
				io.Copy(iop.Stdin, stdin)
				logrus.Debugln("goroutine io.Copy completed - calling iop.Stdin.Close")
				iop.Stdin.Close()
				logrus.Debugln("goroutine iop.Stdin.Close completed, exiting routine")

			}()
		}
	} else {
		if c != nil && !c.Config.Tty {
			// tty is enabled, so dont close containerd's iopipe stdin.
			if iop.Stdin != nil {
				logrus.Debugln("tty is enabled path, calling iop.Stdin.Close")
				iop.Stdin.Close()
			}
		}
	}

	copy := func(w io.Writer, r io.Reader) {
		logrus.Debugln("in copy func - calling s.Add(1)")
		s.Add(1)
		logrus.Debugln("Spinning up goroutine to do the copy")
		go func() {
			logrus.Debugln("Goroutine calling io.Copy(w,r)")
			if _, err := io.Copy(w, r); err != nil {
				logrus.Errorf("%v stream copy error: %v", id, err)
			}
			logrus.Debugln("Gorouting calling s.Done()")
			s.Done()
			logrus.Debugln("End of goroutine calling io.Copy(w,r)")
		}()
	}

	if iop.Stdout != nil {
		logrus.Debugln("Calling copy on s.Stdout(), iop.Stdout")
		copy(s.Stdout(), iop.Stdout)
	}
	if iop.Stderr != nil {
		logrus.Debugln("Calling copy on s.Stderr(), iop.Stderr")
		copy(s.Stderr(), iop.Stderr)
	}

	return nil
}
