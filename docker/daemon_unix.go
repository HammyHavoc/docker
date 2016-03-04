// +build daemon,!windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	apiserver "github.com/docker/docker/api/server"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/system"
)

const defaultDaemonConfigFile = "/etc/docker/daemon.json"

func setPlatformServerConfig(serverConfig *apiserver.Config, daemonCfg *daemon.Config) *apiserver.Config {
	serverConfig.EnableCors = daemonCfg.EnableCors
	serverConfig.CorsHeaders = daemonCfg.CorsHeaders

	return serverConfig
}

// currentUserIsOwner checks whether the current user is the owner of the given
// file.
func currentUserIsOwner(f string) bool {
	if fileInfo, err := system.Stat(f); err == nil && fileInfo != nil {
		if int(fileInfo.UID()) == os.Getuid() {
			return true
		}
	}
	return false
}

// setDefaultUmask sets the umask to 0022 to avoid problems
// caused by custom umask
func setDefaultUmask() error {
	desiredUmask := 0022
	syscall.Umask(desiredUmask)
	if umask := syscall.Umask(desiredUmask); umask != desiredUmask {
		return fmt.Errorf("failed to set umask: expected %#o, got %#o", desiredUmask, umask)
	}

	return nil
}

func getDaemonConfDir() string {
	return "/etc/docker"
}

// setupConfigReloadTrap configures the USR2 signal to reload the configuration.
func setupConfigReloadTrap(configFile string, flags *mflag.FlagSet, reload func(*daemon.Config)) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	go func() {
		for range c {
			if err := daemon.ReloadConfiguration(configFile, flags, reload); err != nil {
				logrus.Error(err)
			}
		}
	}()
}

func (cli *DaemonCli) initLibcontainerd() libcontainerd.Remote {

	remoteOpt := []libcontainerd.RemoteOption{
		libcontainerd.WithDebugLog(cli.Config.Debug),
	}
	if cli.Config.ContainerdAddr != "" {
		remoteOpt = append(remoteOpt, libcontainerd.WithRemoteAddr(cli.Config.ContainerdAddr))
	} else {
		remoteOpt = append(remoteOpt, libcontainerd.WithStartDaemon(true))
	}
	containerdRemote, err := libcontainerd.New(filepath.Join(cli.Config.ExecRoot, "libcontainerd"), remoteOpt...)
	if err != nil {
		logrus.Error(err)
	}
	return containerdRemote
}

func cleanupRemote(cdr libcontainerd.Remote) {
	containerdRemote.Cleanup()
}
