module go-kubectl

go 1.18

require (
	github.com/rs/zerolog v1.29.0
	github.com/steadybit/action-kit/go/action_kit_api/v2 v2.4.3
	github.com/steadybit/action-kit/go/action_kit_sdk v0.0.0-20211116155030-1c1b0c1c1c1c
	github.com/steadybit/extension-kit v1.7.4
)

require (
	github.com/google/uuid v1.3.0 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	golang.org/x/sys v0.6.0 // indirect
)

replace github.com/steadybit/action-kit/go/action_kit_sdk => ../../go/action_kit_sdk
