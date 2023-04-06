# Attack Example: Kubernetes Attacks via kubectl

This attack example showcases how to leverage Go and the [Action Kit Go SDK](../../go/action_kit_sdk/README.md) to expose arbitrary kubectl commands as
attacks – in this case, `kubectl rollout restart`!

The attack example exposes several HTTP endpoints that translate the incoming HTTP requests into executed kubectl commands. The attack is deployed within
Kubernetes as a container that contains the kubectl CLI. Interacting with the kubectl CLI is often more approachable than direct interaction with the Kubernetes
API and therefore lends itself to an example app.

## Starting the example through Kubernetes

This is the recommended approach to give this example app a try. The app is deployed within a namespace `steadybit-extension` as a deployment
called `example-go-kubectl`. Several other resources are created as well to permit access to the relevant API endpoints. For more details, please
inspect `kubernetes.yml`.

```sh
kubectl apply -f kubernetes.yml
```

Once deployed in your Kubernetes cluster the example is reachable
through `http://example-go-kubectl.steadybit-extension.svc.cluster.local:8083/actions`. Steadybit agents can be configured to support this action
extension through the environment variable `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_0_URL`.

## Starting the example from source

**Note:** The app requires `kubectl` to be available on the `$PATH` and to be configured properly.

```sh
go run .
```

## Starting the example through Docker

**Note:** `kubectl` is part of the Docker image, but it is not configured by default.

```sh
docker run --init -p 8083:8083 ghcr.io/steadybit/example-go-kubectl:main
```

