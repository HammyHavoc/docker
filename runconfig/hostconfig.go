package runconfig

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"

	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/docker/pkg/ulimit"
)

// KeyValuePair is a structure that hold a value for a key.
type KeyValuePair struct {
	Key   string
	Value string
}

// NetworkMode represents the container network stack.
type NetworkMode string

// IsolationLevel represents the isolation level of a container. The supported
// values are platform specific
type IsolationLevel string

// IsDefault indicates the default isolation level of a container. On Linux this
// is the native driver. On Windows, this is a Windows Server Container.
func (i IsolationLevel) IsDefault() bool {
	return strings.ToLower(string(i)) == "default" || string(i) == ""
}

// IpcMode represents the container ipc stack.
type IpcMode string

// IsPrivate indicates whether the container uses it's private ipc stack.
func (n IpcMode) IsPrivate() bool {
	return !(n.IsHost() || n.IsContainer())
}

// IsHost indicates whether the container uses the host's ipc stack.
func (n IpcMode) IsHost() bool {
	return n == "host"
}

// IsContainer indicates whether the container uses a container's ipc stack.
func (n IpcMode) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == "container"
}

// Container returns the name of the container ipc stack is going to be used.
func (n IpcMode) Container() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// UTSMode represents the UTS namespace of the container.
type UTSMode string

// IsPrivate indicates whether the container uses it's private UTS namespace.
func (n UTSMode) IsPrivate() bool {
	return !(n.IsHost())
}

// IsHost indicates whether the container uses the host's UTS namespace.
func (n UTSMode) IsHost() bool {
	return n == "host"
}

// PidMode represents the pid stack of the container.
type PidMode string

// IsPrivate indicates whether the container uses it's private pid stack.
func (n PidMode) IsPrivate() bool {
	return !(n.IsHost())
}

// IsHost indicates whether the container uses the host's pid stack.
func (n PidMode) IsHost() bool {
	return n == "host"
}

// DeviceMapping represents the device mapping between the host and the container.
type DeviceMapping struct {
	PathOnHost        string
	PathInContainer   string
	CgroupPermissions string
}

// RestartPolicy represents the restart policies of the container.
type RestartPolicy struct {
	Name              string
	MaximumRetryCount int
}

// IsNone indicates whether the container has the "no" restart policy.
// This means the container will not automatically restart when exiting.
func (rp *RestartPolicy) IsNone() bool {
	return rp.Name == "no"
}

// IsAlways indicates whether the container has the "always" restart policy.
// This means the container will automatically restart regardless of the exit status.
func (rp *RestartPolicy) IsAlways() bool {
	return rp.Name == "always"
}

// IsOnFailure indicates whether the container has the "on-failure" restart policy.
// This means the contain will automatically restart of exiting with a non-zero exit status.
func (rp *RestartPolicy) IsOnFailure() bool {
	return rp.Name == "on-failure"
}

// IsUnlessStopped indicates whether the container has the
// "unless-stopped" restart policy. This means the container will
// automatically restart unless user has put it to stopped state.
func (rp *RestartPolicy) IsUnlessStopped() bool {
	return rp.Name == "unless-stopped"
}

// LogConfig represents the logging configuration of the container.
type LogConfig struct {
	Type   string
	Config map[string]string
}

// HostConfig the non-portable Config structure of a container.
// Here, "non-portable" means "dependent of the host we are running on".
// Portable information *should* appear in Config.
type HostConfig struct {
	Binds             []string              // List of volume bindings for this container
	ContainerIDFile   string                // File (path) where the containerId is written
	Memory            int64                 // Memory limit (in bytes)
	MemoryReservation int64                 // Memory soft limit (in bytes)
	MemorySwap        int64                 // Total memory usage (memory + swap); set `-1` to disable swap
	KernelMemory      int64                 // Kernel memory limit (in bytes)
	CPUShares         int64                 `json:"CpuShares"` // CPU shares (relative weight vs. other containers)
	CPUPeriod         int64                 `json:"CpuPeriod"` // CPU CFS (Completely Fair Scheduler) period
	CpusetCpus        string                // CpusetCpus 0-2, 0,1
	CpusetMems        string                // CpusetMems 0-2, 0,1
	CPUQuota          int64                 `json:"CpuQuota"` // CPU CFS (Completely Fair Scheduler) quota
	BlkioWeight       uint16                // Block IO weight (relative weight vs. other containers)
	OomKillDisable    bool                  // Whether to disable OOM Killer or not
	MemorySwappiness  *int64                // Tuning container memory swappiness behaviour
	Privileged        bool                  // Is the container in privileged mode
	PortBindings      nat.PortMap           // Port mapping between the exposed port (container) and the host
	Links             []string              // List of links (in the name:alias form)
	PublishAllPorts   bool                  // Should docker publish all exposed port for the container
	DNS               []string              `json:"Dns"`        // List of DNS server to lookup
	DNSOptions        []string              `json:"DnsOptions"` // List of DNSOption to look for
	DNSSearch         []string              `json:"DnsSearch"`  // List of DNSSearch to look for
	ExtraHosts        []string              // List of extra hosts
	VolumesFrom       []string              // List of volumes to take from other container
	Devices           []DeviceMapping       // List of devices to map inside the container
	NetworkMode       NetworkMode           // Network namespace to use for the container
	IpcMode           IpcMode               // IPC namespace to use for the container	// Unix specific
	PidMode           PidMode               // PID namespace to use for the container	// Unix specific
	UTSMode           UTSMode               // UTS namespace to use for the container	// Unix specific
	CapAdd            *stringutils.StrSlice // List of kernel capabilities to add to the container
	CapDrop           *stringutils.StrSlice // List of kernel capabilities to remove from the container
	GroupAdd          []string              // List of additional groups that the container process will run as
	RestartPolicy     RestartPolicy         // Restart policy to be used for the container
	SecurityOpt       []string              // List of string values to customize labels for MLS systems, such as SELinux.
	ReadonlyRootfs    bool                  // Is the container root filesystem in read-only	// Unix specific
	Ulimits           []*ulimit.Ulimit      // List of ulimits to be set in the container
	LogConfig         LogConfig             // Configuration of the logs for this container
	CgroupParent      string                // Parent cgroup.
	ConsoleSize       [2]int                // Initial console size on Windows
	VolumeDriver      string                // Name of the volume driver used to mount volumes
	Isolation         IsolationLevel        // Isolation level of the container (eg default, hyperv)
}

// DecodeHostConfig creates a HostConfig based on the specified Reader.
// It assumes the content of the reader will be JSON, and decodes it.
func DecodeHostConfig(src io.Reader) (*HostConfig, error) {
	decoder := json.NewDecoder(src)

	var w ContainerConfigWrapper
	if err := decoder.Decode(&w); err != nil {
		return nil, err
	}

	hc := w.getHostConfig()

	// JJH Not sure about this. Think it might be useful.
	if err := validateHostConfigPlatformFields(hc); err != nil {
		return nil, err
	}
	return hc, nil
}

// SetDefaultNetModeIfBlank changes the NetworkMode in a HostConfig structure
// to default if it is not populated. This ensures backwards compatibility after
// the validation of the network mode was moved from the docker CLI to the
// docker daemon.
func SetDefaultNetModeIfBlank(hc *HostConfig) *HostConfig {
	if hc != nil {
		if hc.NetworkMode == NetworkMode("") {
			hc.NetworkMode = NetworkMode("default")
		}
	}
	return hc
}

// validateHostConfigPlatformFields examines the fields passed in the API, to
// ensure that fields which are not supported on the platform have not been set.
func validateHostConfigPlatformFields(hc *HostConfig) error {

	v := reflect.ValueOf(*hc)
	typeOf := v.Type()
	if typeOf.Kind() == reflect.Ptr {
		typeOf = typeOf.Elem()
	}

	for i := 0; i < v.NumField(); i++ {
		name := typeOf.Field(i).Name
		value := v.Field(i).Interface
		errPopulated := "HostConfig '%s' has value %#v. Not supported on " + runtime.GOOS

		switch name {

		//
		// PLATFORM AGNOSTIC FIELDS (alphabetical)
		//

		case "Binds",
			"ContainerIDFile",
			"CPUShares",
			"LogConfig",
			"NetworkMode",
			"PortBindings",
			"RestartPolicy",
			"VolumeDriver",
			"VolumesFrom":
			break

		//
		// UNIX SPECIFIC FIELDS (alphabetical)
		//

		case "BlkioWeight":
			if runtime.GOOS == "windows" && hc.BlkioWeight != 0 {
				return fmt.Errorf(errPopulated, name, hc.BlkioWeight)
			}
			break

		case "CapAdd":
			if runtime.GOOS == "windows" && hc.CapAdd != nil {
				return fmt.Errorf(errPopulated, name, hc.CapAdd)
			}
			break

		case "CapDrop":
			if runtime.GOOS == "windows" && hc.CapDrop != nil {
				return fmt.Errorf(errPopulated, name, hc.CapDrop)
			}
			break

		case "CgroupParent":
			if runtime.GOOS == "windows" && hc.CgroupParent != "" {
				return fmt.Errorf(errPopulated, name, hc.CgroupParent)
			}
			break

		case "CPUPeriod":
			if runtime.GOOS == "windows" && hc.CPUPeriod != 0 {
				return fmt.Errorf(errPopulated, name, hc.CPUPeriod)
			}
			break

		case "CPUQuota":
			if runtime.GOOS == "windows" && hc.CPUQuota != 0 {
				return fmt.Errorf(errPopulated, name, hc.CPUQuota)
			}
			break

		case "CpusetCpus":
			if runtime.GOOS == "windows" && hc.CpusetCpus != "" {
				return fmt.Errorf(errPopulated, name, hc.CpusetCpus)
			}
			break

		case "CpusetMems":
			if runtime.GOOS == "windows" && hc.CpusetMems != "" {
				return fmt.Errorf(errPopulated, name, hc.CpusetMems)
			}
			break

		case "Devices":
			if runtime.GOOS == "windows" && len(hc.Devices) > 0 {
				return fmt.Errorf(errPopulated, name, hc.Devices)
			}
			break

		case "DNS":
			if runtime.GOOS == "windows" && len(hc.DNS) > 0 {
				for i := 0; i < len(hc.DNS); i++ {
					if len(hc.DNS[i]) > 0 {
						return fmt.Errorf(errPopulated, name, hc.DNS)
					}
				}
			}
			break

		case "DNSOptions":
			if runtime.GOOS == "windows" && len(hc.DNSOptions) > 0 {
				for i := 0; i < len(hc.DNSOptions); i++ {
					if len(hc.DNSOptions[i]) > 0 {
						return fmt.Errorf(errPopulated, name, hc.DNSOptions)
					}
				}
			}
			break

		case "DNSSearch":
			if runtime.GOOS == "windows" && len(hc.DNSSearch) > 0 {
				for i := 0; i < len(hc.DNSSearch); i++ {
					if len(hc.DNSSearch[i]) > 0 {
						return fmt.Errorf(errPopulated, name, hc.DNSSearch)
					}
				}
			}
			break

		case "ExtraHosts":
			if runtime.GOOS == "windows" && len(hc.ExtraHosts) > 0 {
				for i := 0; i < len(hc.ExtraHosts); i++ {
					if len(hc.ExtraHosts[i]) > 0 {
						return fmt.Errorf(errPopulated, name, hc.ExtraHosts)
					}
				}
			}
			break

		case "GroupAdd":
			if runtime.GOOS == "windows" && hc.GroupAdd != nil {
				return fmt.Errorf(errPopulated, name, hc.GroupAdd)
			}
			break

		case "IpcMode":
			if !hc.IpcMode.Valid() {
				if runtime.GOOS == "windows" {
					return fmt.Errorf(errPopulated, name, hc.IpcMode)
				} else {
					return fmt.Errorf("invalid IPC mode")
				}
			}
			break

		case "KernelMemory":
			if runtime.GOOS == "windows" && hc.KernelMemory != 0 {
				return fmt.Errorf(errPopulated, name, hc.KernelMemory)
			}
			break

		case "Links":
			if runtime.GOOS == "windows" && len(hc.Links) > 0 {
				for i := 0; i < len(hc.Links); i++ {
					if len(hc.Links[i]) > 0 {
						return fmt.Errorf(errPopulated, name, hc.Links)
					}
				}
			}
			break

		case "Memory":
			if runtime.GOOS == "windows" && hc.Memory != 0 {
				return fmt.Errorf(errPopulated, name, hc.Memory)
			}
			break

		case "MemoryReservation":
			if runtime.GOOS == "windows" && hc.MemoryReservation != 0 {
				return fmt.Errorf(errPopulated, name, hc.MemoryReservation)
			}
			break

		case "MemorySwap":
			if runtime.GOOS == "windows" && hc.MemorySwap != 0 {
				return fmt.Errorf(errPopulated, name, hc.MemorySwap)
			}
			break

		case "MemorySwappiness":
			// Note defaults to -1 in CLI, but allow 0 for direct REST caller.
			if runtime.GOOS == "windows" && hc.MemorySwappiness != nil && *hc.MemorySwappiness > 0 {
				if hc.MemorySwappiness == nil {
					return fmt.Errorf(errPopulated, name, nil)
				}
				return fmt.Errorf(errPopulated, name, *hc.MemorySwappiness)
			}
			break

		case "OomKillDisable":
			if runtime.GOOS == "windows" && hc.OomKillDisable {
				return fmt.Errorf(errPopulated, name, hc.OomKillDisable)
			}
			break

		case "PidMode":
			if !hc.PidMode.Valid() {
				if runtime.GOOS == "windows" {
					return fmt.Errorf(errPopulated, name, hc.PidMode)
				} else {
					return fmt.Errorf("invalid PID mode")
				}
			}
			break

		case "Privileged":
			if runtime.GOOS == "windows" && hc.Privileged {
				return fmt.Errorf(errPopulated, name, hc.Privileged)
			}
			break

		case "PublishAllPorts":
			if runtime.GOOS == "windows" && hc.PublishAllPorts {
				return fmt.Errorf(errPopulated, name, hc.PublishAllPorts)
			}
			break

		case "ReadonlyRootfs":
			if runtime.GOOS == "windows" && hc.ReadonlyRootfs {
				return fmt.Errorf(errPopulated, name, hc.ReadonlyRootfs)
			}
			break

		case "SecurityOpt":
			if runtime.GOOS == "windows" && len(hc.SecurityOpt) > 0 {
				for i := 0; i < len(hc.SecurityOpt); i++ {
					if len(hc.SecurityOpt[i]) > 0 {
						return fmt.Errorf(errPopulated, name, hc.SecurityOpt)
					}
				}
			}
			break

		case "Ulimits":
			if runtime.GOOS == "windows" && len(hc.Ulimits) > 0 {
				return fmt.Errorf(errPopulated, name, hc.Ulimits)
			}
			break

		case "UTSMode":
			if !hc.UTSMode.Valid() {
				if runtime.GOOS == "windows" {
					return fmt.Errorf(errPopulated, name, hc.UTSMode)
				} else {
					return fmt.Errorf("invalid UTS mode")
				}
			}
			break

		//
		// WINDOWS SPECIFIC FIELDS (alphabetical)
		//

		// ConsoleSize is Windows client specific. However, we can't validate it, it is just ignored by other platforms
		case "ConsoleSize":
			break

		// Isolation is valid only on Windows, but allow empty and "default"
		case "Isolation":
			if runtime.GOOS != "windows" && !IsolationLevel(hc.Isolation).IsDefault() {
				return fmt.Errorf(errPopulated, name, hc.Isolation)
			}
			break

		//
		// EVERYTHING ELSE
		//

		// In case the HostConfig is extended, but no validation check is added
		default:
			return fmt.Errorf("Unrecognised HostConfig field '%s' has value %#v", name, value)
		}
	}

	return nil
}
