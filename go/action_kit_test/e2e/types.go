package e2e

const (
	RuntimeContainerd         Runtime = "containerd"
	DefaultSocketContainerd           = "/run/containerd/containerd.sock"
	DefaultRuncRootContainerd         = "/run/containerd/runc/k8s.io"
	RuntimeDocker             Runtime = "docker"
	DefaultSocketDocker               = "/var/run/docker.sock"
	DefaultRuncRootDocker             = "/run/docker/runtime-runc/moby"
	RuntimeCrio               Runtime = "cri-o"
	DefaultSocketCrio                 = "/var/run/crio/crio.sock"
	DefaultRuncRootCrio               = "/run/runc"
)

var (
	AllRuntimes = []Runtime{RuntimeDocker, RuntimeContainerd, RuntimeCrio}
)

type Runtime string

func (runtime Runtime) DefaultSocket() string {
	switch runtime {
	case RuntimeDocker:
		return DefaultSocketDocker
	case RuntimeContainerd:
		return DefaultSocketContainerd
	case RuntimeCrio:
		return DefaultSocketCrio
	}
	return ""
}

func (runtime Runtime) DefaultRuncRoot() string {
	switch runtime {
	case RuntimeDocker:
		return DefaultRuncRootDocker
	case RuntimeContainerd:
		return DefaultRuncRootContainerd
	case RuntimeCrio:
		return DefaultRuncRootCrio
	}
	return ""
}
