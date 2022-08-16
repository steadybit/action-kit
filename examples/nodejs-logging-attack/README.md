# Attack Example: Logging

updateA tiny attack implementation which exposes the required HTTP APIs and logs all interactions with it to the console. If you are new to Steadybit's attacks,
this
example app will help you understand the fundamental contracts and control flows.

The attack uses targets with the target-type `cat`. These are targets from
our [nodejs-example-discovery](https://github.com/steadybit/discovery-kit/tree/main/examples/nodejs-example-discovery).

## Starting the example

```sh
npm install
npm start
```

## Starting the example through Kubernetes

This is the recommended approach to give this example app a try. The app is deployed within a namespace `example-nodejs-logging-attack` as a deployment
called `example-nodejs-logging-attack`. Several other resources are created as well to permit access to the relevant API endpoints. For more details, please
inspect `kubernetes.yml`.

```sh
kubectl apply -f kubernetes.yml
```

Once deployed in your Kubernetes cluster the example is reachable
through `http://example-nodejs-logging-attack.example-nodejs-logging-attack.svc.cluster.local:8084/actions`. Steadybit agents can be configured to support this
attack provider through the environment variable `STEADYBIT_AGENT_ATTACKS_EXTENSIONS_0_URL`.

## Starting the example using Docker

```sh
docker run -it \
  --rm \
  --init \
  -p 8084:8084 \
  ghcr.io/steadybit/example-nodejs-logging-attack:main
```