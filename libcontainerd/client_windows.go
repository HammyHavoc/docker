package libcontainerd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/libcontainerd/windowsoci"
)

// TODO Windows containerd (@jhowardmsft) Make common/platform specific
type client struct {
	sync.Mutex                              // lock for containerMutexes map access
	mapMutex         sync.RWMutex           // protects read/write oprations from containers map
	containerMutexes map[string]*sync.Mutex // lock by container ID
	backend          Backend
	remote           *remote
	containers       map[string]*container
}

// defaultContainerNAT is the default name of the container NAT device that is
// preconfigured on the server.
const defaultContainerNAT = "ContainerNAT"

// Win32 error codes that are used for various workarounds
// These really should be ALL_CAPS to match golangs syscall library and standard
// Win32 error conventions, but golint insists on CamelCase.
const (
	CoEClassstring     = syscall.Errno(0x800401F3) // Invalid class string
	ErrorNoNetwork     = syscall.Errno(1222)       // The network is not present or not started
	ErrorBadPathname   = syscall.Errno(161)        // The specified path is invalid
	ErrorInvalidObject = syscall.Errno(0x800710D8) // The object identifier does not represent a valid object
)

type layer struct {
	ID   string
	Path string
}

type defConfig struct {
	DefFile string
}

type portBinding struct {
	Protocol     string
	InternalPort int
	ExternalPort int
}

type natSettings struct {
	Name         string
	PortBindings []portBinding
}

type networkConnection struct {
	NetworkName string
	// TODO Windows: Add Ip4Address string to this structure when hooked up in
	// docker CLI. This is present in the HCS JSON handler.
	EnableNat bool
	Nat       natSettings
}
type networkSettings struct {
	MacAddress string
}

type device struct {
	DeviceType string
	Connection interface{}
	Settings   interface{}
}

type mappedDir struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

type containerInit struct {
	SystemType              string      // HCS requires this to be hard-coded to "Container"
	Name                    string      // Name of the container. We use the docker ID.
	Owner                   string      // The management platform that created this container
	IsDummy                 bool        // Used for development purposes.
	VolumePath              string      // Windows volume path for scratch space
	Devices                 []device    // Devices used by the container
	IgnoreFlushesDuringBoot bool        // Optimization hint for container startup in Windows
	LayerFolderPath         string      // Where the layer folders are located
	Layers                  []layer     // List of storage layers
	ProcessorWeight         uint64      `json:",omitempty"` // CPU Shares 0..10000 on Windows; where 0 will be omitted and HCS will default.
	HostName                string      // Hostname
	MappedDirectories       []mappedDir // List of mapped directories (volumes/mounts)
	SandboxPath             string      // Location of unmounted sandbox (used for Hyper-V containers, not Windows Server containers)
	HvPartition             bool        // True if it a Hyper-V Container
}

// defaultOwner is a tag passed to HCS to allow it to differentiate between
// container creator management stacks. We hard code "docker" in the case
// of docker.
const defaultOwner = "docker"

func (c *client) Create(id string, spec Spec, options ...CreateOption) error {

	logrus.Debugln("LCD Create() with spec", spec)

	// TODO Windows TP5 timeframe. Remove this once TP4 is no longer supported.
	// Hack for TP4.
	// This overcomes an issue on TP4 which causes CreateComputeSystem to
	// intermittently fail. It's predominantly here to make Windows to Windows
	// CI more reliable.
	tp4RetryHack := hcsshim.IsTP4()

	cu := &containerInit{
		SystemType: "Container",
		Name:       id,
		Owner:      defaultOwner,

		VolumePath:              spec.Root.Path,
		IgnoreFlushesDuringBoot: spec.Windows.FirstStart,
		LayerFolderPath:         spec.Windows.LayerFolder,
		HostName:                spec.Hostname,
	}

	if spec.Windows.Resources != nil && spec.Windows.Resources.CPU != nil {
		cu.ProcessorWeight = *spec.Windows.Resources.CPU.Shares
	}

	if spec.Windows.HvRuntime != nil {
		cu.HvPartition = len(spec.Windows.HvRuntime.ImagePath) > 0
	}

	if cu.HvPartition {
		cu.SandboxPath = filepath.Dir(spec.Windows.LayerFolder)
	} else {
		cu.VolumePath = spec.Root.Path
		cu.LayerFolderPath = spec.Windows.LayerFolder
	}

	for _, layerPath := range spec.Windows.LayerPaths {
		_, filename := filepath.Split(layerPath)
		g, err := hcsshim.NameToGuid(filename)
		if err != nil {
			return err
		}
		cu.Layers = append(cu.Layers, layer{
			ID:   g.ToString(),
			Path: layerPath,
		})
	}

	// Add the mounts (volumes, bind mounts etc) to the structure
	mds := make([]mappedDir, len(spec.Mounts))
	for i, mount := range spec.Mounts {
		mds[i] = mappedDir{
			HostPath:      mount.Source,
			ContainerPath: mount.Destination,
			ReadOnly:      mount.Readonly}
	}
	cu.MappedDirectories = mds

	// TODO Windows. At some point, when there is CLI on docker run to
	// enable the IP Address of the container to be passed into docker run,
	// the IP Address needs to be wired through to HCS in the JSON. It
	// would be present in c.Network.Interface.IPAddress. See matching
	// TODO in daemon\container_windows.go, function populateCommand.

	if spec.Windows.Networking != nil {

		var pbs []portBinding

		// Enumerate through the port bindings specified by the user and convert
		// them into the internal structure matching the JSON blob that can be
		// understood by the HCS.
		for i, v := range spec.Windows.Networking.PortBindings {
			proto := strings.ToUpper(i.Proto())
			if proto != "TCP" && proto != "UDP" {
				return fmt.Errorf("invalid protocol %s", i.Proto())
			}

			if len(v) > 1 {
				return fmt.Errorf("Windows does not support more than one host port in NAT settings")
			}

			for _, v2 := range v {
				var (
					iPort, ePort int
					err          error
				)
				if len(v2.HostIP) != 0 {
					return fmt.Errorf("Windows does not support host IP addresses in NAT settings")
				}
				if ePort, err = strconv.Atoi(v2.HostPort); err != nil {
					return fmt.Errorf("invalid container port %s: %s", v2.HostPort, err)
				}
				if iPort, err = strconv.Atoi(i.Port()); err != nil {
					return fmt.Errorf("invalid internal port %s: %s", i.Port(), err)
				}
				if iPort < 0 || iPort > 65535 || ePort < 0 || ePort > 65535 {
					return fmt.Errorf("specified NAT port is not in allowed range")
				}
				pbs = append(pbs,
					portBinding{ExternalPort: ePort,
						InternalPort: iPort,
						Protocol:     proto})
			}
		}

		// TODO Windows: TP3 workaround. Allow the user to override the name of
		// the Container NAT device through an environment variable. This will
		// ultimately be a global daemon parameter on Windows, similar to -b
		// for the name of the virtual switch (aka bridge).
		cn := os.Getenv("DOCKER_CONTAINER_NAT")
		if len(cn) == 0 {
			cn = defaultContainerNAT
		}

		dev := device{
			DeviceType: "Network",
			Connection: &networkConnection{
				NetworkName: spec.Windows.Networking.Bridge,
				// TODO Windows: Fixme, next line. Needs HCS fix.
				EnableNat: false,
				Nat: natSettings{
					Name:         cn,
					PortBindings: pbs,
				},
			},
		}

		if spec.Windows.Networking.MacAddress != "" {
			windowsStyleMAC := strings.Replace(
				spec.Windows.Networking.MacAddress, ":", "-", -1)
			dev.Settings = networkSettings{
				MacAddress: windowsStyleMAC,
			}
		}
		cu.Devices = append(cu.Devices, dev)
	} else {
		logrus.Debugln("No network interface")
	}

	configurationb, err := json.Marshal(cu)
	if err != nil {
		return err
	}

	configuration := string(configurationb)

	// TODO Windows TP5 timeframe. Remove when TP4 is no longer supported.
	// The following a workaround for Windows TP4 which has a networking
	// bug which fairly frequently returns an error. Back off and retry.
	maxAttempts := 5
	for i := 0; i < maxAttempts; i++ {
		err = hcsshim.CreateComputeSystem(id, configuration)
		if err == nil {
			break
		}

		if !tp4RetryHack {
			return err
		}

		if herr, ok := err.(*hcsshim.HcsError); ok {
			if herr.Err != syscall.ERROR_NOT_FOUND && // Element not found
				herr.Err != syscall.ERROR_FILE_NOT_FOUND && // The system cannot find the file specified
				herr.Err != ErrorNoNetwork && // The network is not present or not started
				herr.Err != ErrorBadPathname && // The specified path is invalid
				herr.Err != CoEClassstring && // Invalid class string
				herr.Err != ErrorInvalidObject { // The object identifier does not represent a valid object
				logrus.Debugln("Failed to create temporary container ", err)
				return err
			}
			logrus.Warnf("Invoking Windows TP4 retry hack (%d of %d)", i, maxAttempts-1)
			time.Sleep(50 * time.Millisecond)
		}
	}

	container := c.newContainer(id, spec.Process, options...)

	defer func() {
		if err != nil {
			c.deleteContainer(id)
		}
	}()

	logrus.Debugf("Finished Create() id=%s, calling container.start()", id)
	return container.start()
}

// TODO Implement
func (c *client) AddProcess(id, processID string, specp Process) error {
	return nil
}

// TODO Implment
func (c *client) Signal(id string, sig int) error {
	return nil
}

// TODO Implement
func (c *client) Resize(id, processID string, width, height int) error {
	return nil
}

// TODO Implement (error on Windows)
func (c *client) Pause(id string) error {
	return nil
}

// TODO Implement
func (c *client) Resume(id string) error {
	return nil
}

// TODO Implement (error on Windows for now)
func (c *client) Stats(id string) (*Stats, error) {
	return nil, nil
}

// TODO Implement
func (c *client) Restore(id string, options ...CreateOption) error {
	return nil
}

// TODO Implement
func (c *client) GetPidsForContainer(id string) ([]int, error) {
	return nil, nil
}

// TODO Implement
func (c *client) UpdateResources(id string, resources Resources) error {
	return nil
}

func (c *client) newContainer(id string, p windowsoci.Process, options ...CreateOption) *container {
	container := &container{
		process: process{
			id:           id,
			client:       c,
			friendlyName: initFriendlyName,
			ociProcess:   p,
		},
		processes: make(map[string]*process),
	}

	// BUGBUG TODO Windows containerd. What options?
	//	for _, option := range options {
	//		if err := option.Apply(container); err != nil {
	//			logrus.Error(err)
	//		}
	//	}

	return container
}
