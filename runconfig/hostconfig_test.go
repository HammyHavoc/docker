package runconfig

import (
	//	"bytes"
	//"fmt"
	//	"io/ioutil"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/pkg/ulimit"
)

func TestRestartPolicy(t *testing.T) {
	restartPolicies := map[RestartPolicy][]bool{
		// none, always, failure
		RestartPolicy{}:                {false, false, false},
		RestartPolicy{"something", 0}:  {false, false, false},
		RestartPolicy{"no", 0}:         {true, false, false},
		RestartPolicy{"always", 0}:     {false, true, false},
		RestartPolicy{"on-failure", 0}: {false, false, true},
	}
	for restartPolicy, state := range restartPolicies {
		if restartPolicy.IsNone() != state[0] {
			t.Fatalf("RestartPolicy.IsNone for %v should have been %v but was %v", restartPolicy, state[0], restartPolicy.IsNone())
		}
		if restartPolicy.IsAlways() != state[1] {
			t.Fatalf("RestartPolicy.IsAlways for %v should have been %v but was %v", restartPolicy, state[1], restartPolicy.IsAlways())
		}
		if restartPolicy.IsOnFailure() != state[2] {
			t.Fatalf("RestartPolicy.IsOnFailure for %v should have been %v but was %v", restartPolicy, state[2], restartPolicy.IsOnFailure())
		}
	}
}

func TestValidateNonPlatformFields(t *testing.T) {

	// Common fields
	testValidateNonPlatformFieldsHelper(t, &HostConfig{Binds: []string{"/host:/container:mode"}}, "Binds", false)
	testValidateNonPlatformFieldsHelper(t, &HostConfig{ContainerIDFile: "/path"}, "ContainerIDFile", false)
	testValidateNonPlatformFieldsHelper(t, &HostConfig{CPUShares: 8765}, "CPUShares", false)
	testValidateNonPlatformFieldsHelper(t, &HostConfig{LogConfig: LogConfig{"something", nil}}, "LogConfig", false)
	pm := make(map[nat.Port][]nat.PortBinding)
	pm["22/tcp"] = nil
	testValidateNonPlatformFieldsHelper(t, &HostConfig{PortBindings: pm}, "LogConfig", false)
	testValidateNonPlatformFieldsHelper(t, &HostConfig{RestartPolicy: RestartPolicy{"restart policy", 5}}, "RestartPolicy", false)
	testValidateNonPlatformFieldsHelper(t, &HostConfig{VolumeDriver: "driver"}, "VolumeDriver", false)
	testValidateNonPlatformFieldsHelper(t, &HostConfig{VolumesFrom: []string{"volfrom"}}, "VolumesFrom", false)

	// Unix fields
	testValidateNonPlatformFieldsHelper(t, &HostConfig{BlkioWeight: 1234}, "BlkioWeight", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{CapAdd: stringutils.NewStrSlice("NET_ADMIN")}, "CapAdd", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{CapDrop: stringutils.NewStrSlice("NET_ADMIN")}, "CapDrop", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{CgroupParent: "cgp"}, "CgroupParent", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{CPUPeriod: 2345}, "CPUPeriod", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{CPUQuota: 3456}, "CPUQuota", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{CpusetCpus: "5,6"}, "CpusetCpus", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{CpusetMems: "700,800"}, "CpusetMems", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{Devices: []DeviceMapping{{"/host", "/container", "rw"}}}, "CpusetMems", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{DNS: []string{"some.suffix.com"}}, "DNS", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{DNSOptions: []string{"an option"}}, "DNSOptions", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{DNSSearch: []string{"search.com"}}, "DNSSearch", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{ExtraHosts: []string{"name1", "name2"}}, "ExtraHosts", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{GroupAdd: []string{"group1", "group2"}}, "GroupAdd", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{IpcMode: "ipcmode"}, "IpcMode", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{IpcMode: "host"}, "IpcMode", (runtime.GOOS != "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{KernelMemory: 4567}, "KernelMemory", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{Links: []string{"link1", "link2"}}, "Links", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{Memory: 5678}, "Memory", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{MemoryReservation: 7890}, "MemoryReservation", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{MemorySwap: 8901}, "MemorySwap", (runtime.GOOS == "windows"))
	var ms int64 = 9012
	testValidateNonPlatformFieldsHelper(t, &HostConfig{MemorySwappiness: &ms}, "MemorySwappiness", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{OomKillDisable: true}, "OomKillDisable", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{PidMode: "pidmode"}, "PidMode", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{PidMode: "host"}, "PidMode", (runtime.GOOS != "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{Privileged: true}, "Priviliged", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{PublishAllPorts: true}, "PublishAllPorts", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{ReadonlyRootfs: true}, "ReadonlyRootfs", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{SecurityOpt: []string{"sopt1", "sopt2"}}, "SecurityOpt", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{Ulimits: []*ulimit.Ulimit{&ulimit.Ulimit{"name", 123, 456}}}, "Ulimit", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{UTSMode: "utsmode"}, "UTSMode", (runtime.GOOS == "windows"))
	testValidateNonPlatformFieldsHelper(t, &HostConfig{UTSMode: "host"}, "UTSMode", (runtime.GOOS != "windows"))

	// Windows Fields
	testValidateNonPlatformFieldsHelper(t, &HostConfig{ConsoleSize: [2]int{80, 25}}, "ConsoleSize", false)
	testValidateNonPlatformFieldsHelper(t, &HostConfig{Isolation: "hyperv"}, "Isolation", (runtime.GOOS != "windows"))
}

func testValidateNonPlatformFieldsHelper(t *testing.T, hc *HostConfig, field string, shouldFail bool) {
	if shouldFail {
		if err := validateHostConfigPlatformFields(hc); err == nil {
			t.Fatalf("Expected %q to fail", field)
		} else {
			if !strings.Contains(err.Error(), "'"+field+"'") && !strings.Contains(err.Error(), "Not supported on "+runtime.GOOS) {
				t.Fatalf("Expect %q to fail on %s. Got %v", field, runtime.GOOS, err)
			}
		}
	} else {
		if err := validateHostConfigPlatformFields(hc); err != nil {
			t.Fatalf("Expect %q to succeed on %s. %s", field, runtime.GOOS, err.Error())
		}
	}
}
