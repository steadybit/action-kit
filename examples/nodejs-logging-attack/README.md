# Custom Attack Example: Logging

A tiny custom attack implementation which exposes the required HTTP APIs and logs all interactions with it to the console. If you are new to Steadybit's custom
attacks, this example app will help you understand the fundamental contracts and control flows.

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
  -p 3001:3001 \
  ghcr.io/steadybit/example-nodejs-logging-attack:main
```