package e2e

const (
	RuntimeContainerd         Runtime = "containerd"
	RuntimeDocker             Runtime = "docker"
	RuntimeCrio               Runtime = "cri-o"
)

var (
	AllRuntimes = []Runtime{RuntimeDocker, RuntimeContainerd, RuntimeCrio}
)

type Runtime string
