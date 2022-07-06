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

## Starting the example using Docker

```sh
docker run -it \
  --rm \
  --init \
  -p 8084:8084 \
  ghcr.io/steadybit/example-nodejs-logging-attack:main
```