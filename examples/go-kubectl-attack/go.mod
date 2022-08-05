module go-kubectl-attack

go 1.18

require (
	github.com/rs/zerolog v1.27.0
	github.com/steadybit/attack-kit/go/attack_kit_api v0.1.0
)

require (
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/steadybit/extension-kit v1.0.1
	golang.org/x/sys v0.0.0-20210927094055-39ccf1dd6fa6 // indirect
)

replace github.com/steadybit/attack-kit/go/attack_kit_api => ../../go/attack_kit_api
