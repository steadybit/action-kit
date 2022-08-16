# ActionKit Go API

This module exposes Go types that you will find helpful when implementing an ActionKit extension.

The types are generated automatically from the ActionKit [OpenAPI specification](https://github.com/steadybit/action-kit/tree/main/openapi).

## Installation

Add the following to your `go.mod` file:

```
go get github.com/steadybit/action-kit/go/action_kit_api@v0.1.0
```

## Usage

```go
import (
	"github.com/steadybit/action-kit/go/action_kit_api"
)

attackList := attack_kit_api.AttackList{
    Attacks: []attack_kit_api.DescribingEndpointReference{
        {
            "GET",
            "/actions/rollout-restart",
        },
    },
}
```