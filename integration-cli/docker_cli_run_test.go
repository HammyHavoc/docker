package main

import (
	"github.com/go-check/check"
)

// pinging Google's DNS resolver should fail when we disable the networking
func (s *DockerSuite) TestRunWithoutNetworking(c *check.C) {
	count := "-c"
	image := "busybox"
	if daemonPlatform == "windows" {
		count = "-n"
		image = WindowsBaseImage
	}

	// First using the long form --net
	out, exitCode, err := dockerCmdWithError("run", "--net=none", image, "ping", count, "1", "8.8.8.8")
	if err != nil && exitCode != 1 {
		c.Fatal(out, err)
	}
	if exitCode != 1 {
		c.Errorf("--net=none should've disabled the network; the container shouldn't have been able to ping 8.8.8.8")
	}
}

// Issue #4681
func (s *DockerSuite) TestRunLoopbackWhenNetworkDisabled(c *check.C) {
	if daemonPlatform == "windows" {
		dockerCmd(c, "run", "--net=none", WindowsBaseImage, "ping", "-n", "1", "127.0.0.1")
	} else {
		dockerCmd(c, "run", "--net=none", "busybox", "ping", "-c", "1", "127.0.0.1")
	}
}
