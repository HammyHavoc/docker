// +build linux freebsd

package daemon

import (
	"github.com/docker/docker/container"
	"github.com/docker/docker/libcontainerd"
)

func (daemon *Daemon) platformStart(container *container.Container) error {
	spec, err := daemon.createSpec(container)
	if err != nil {
		return err
	}

	if err := daemon.containerd.Create(container.ID, *spec, libcontainerd.WithRestartManager(container.RestartManager(true))); err != nil {
		container.Reset(false)
		return err
	}
	return nil
}
