# Steadybit Attak API

This repository represents a **work-in-progress** documentation and implementation of Steadybit's custom attack mechanism. If you are curious to learn more,
[reach out to us](https://www.steadybit.com/contact).

> **To Do**: Create json schema for the api.

## Registering and Discovering Attacks

New attacks are discovered from attack index(es) queried by http. These indexes must be declared to the agents using environment variables:

```shell
# Attack index to query
STEADYBIT_AGENT_ATTACKS_CUSTOM_0_URL=http://custom-attacks-A

STEADYBIT_AGENT_ATTACKS_CUSTOM_1_URL=http://custom-attacks-B
STEADYBIT_AGENT_ATTACKS_CUSTOM_1_METHOD=GET #Http verb used to query the index (default: GET)
STEADYBIT_AGENT_ATTACKS_CUSTOM_1_BASIC_USERNAME=<user> #Username used for Basic Authentication  
STEADYBIT_AGENT_ATTACKS_CUSTOM_1_BASIC_PASSWORD=<password> #Password used for Basic Authentication
```

[Example Index Response](./typescript-api/api.d.ts#L11):

```json
{
  "attacks": [
    {
      "path": "/attacks/logging"
    }
  ]
}
```

## Attack Description

Each path from the index response is queried to describe each attack. Beside `name` and `description`, this defines which targets can be attacked and what
parameters can be configured by the user. The `prepare`, `start` and `stop` properties specify the endpoint to be called for each action.

[Example Describe Attack Response](./typescript-api/api.d.ts#L15):

```json
{
  "id": "logging-attack",
  "name": "Logging Attack",
  "description": "Prints the received payload to the console to illustrate the custom attack API.",
  "version": "1.0.0",
  "category": "resource",
  "target": "container",
  "parameters": [
    {
      "name": "text",
      "label": "Text",
      "type": "string",
      "required": true
    }
  ],
  "prepare": {
    "path": "/attacks/logging/prepare"
  },
  "start": {
    "path": "/attacks/logging/start"
  },
  "stop": {
    "path": "/attacks/logging/stop"
  }
}
```

### Attack Execution

The Attack execution is divided into three steps: `prepare`, `start` and `stop`:

1) The `prepare` step is called with configuration and target to attack and must return `200 OK` and a json body with a state object which is then passed
   the `start` and `stop` steps.
2) The `start` step is called with the state returned by the `prepare` step. Must return `200 OK` on success or `500 Server Error` on failure. May also return a
   json body with a state object which is then passed the `stop` step.
3) The `stop` step is called with the state returned by the `prepare`/`start` step. Must return `200 OK` on success or `500 Server Error`on failure.

> **Note**: The `stop` request will also be issued if the `start` request fails. In case of timeout for `start` we can't tell if the attack was started or not, therefore a `stop` request is issued. So the state should contain all data to start and stop the request.

> **TBD**: How to deal with time control? There are 3 scenarios to consider:
> 1. One-Shots (e.g. killing processes)
> 2. Start, wait a certain time and then stop (e.g. network blackhole) - steadybit agent controls timing.
> 2. Start and wait for finish (e.g. service rollover) - waits for the external action to finish.

> **TBD**: How to transport logs and/or error messages

#### Prepare

[Example Prepare Request](./typescript-api/api.d.ts#L65):

```json
{
  "target": {
    "name": "docker://750d1998547f24f0ffab7f768f471ce4f25cf8c0528eedeec79338fdf88e29fb",
    "attributes": {
      "container.engine": [
        "docker"
      ],
      "container.port": [
        "5432:5432"
      ],
      "container.name": [
        "postgres_cm"
      ],
      "label.org.testcontainers.sessionid": [
        "9c250ffd-f197-4f24-ae60-7d7e4893677e"
      ],
      "label": [
        "org.testcontainers"
      ],
      "container.image": [
        "postgres:13"
      ],
      "container.ipv4": [
        "172.17.0.4"
      ],
      "agent.hostname": [
        "joshiste-mbp"
      ],
      "container.host": [
        "docker-desktop"
      ],
      "container.id": [
        "docker://750d1998547f24f0ffab7f768f471ce4f25cf8c0528eedeec79338fdf88e29fb"
      ]
    }
  },
  "config": {
    "text": "test",
    "level": "info"
  }
}

```

[Example Prepare Response](./typescript-api/api.d.ts#L73):

```json
{
  "state": {
    "text": "test",
    "level": "info"
  }
}
```

#### Start

[Example Start Request](./typescript-api/api.d.ts#L77):

```json
{
  "state": {
    "text": "test",
    "level": "info"
  }
}
```

[Example Start Response](./typescript-api/api.d.ts#L81):

```json
{
  "state": {
    "text": "test",
    "level": "info"
  }
}
```

#### Stop

[Example Stop Request](./typescript-api/api.d.ts#L85):

```json
{
  "state": {
    "text": "test",
    "level": "info"
  }
}
```

## Example

If you want to get started, we suggest to start with the [logging attack](https://github.com/steadybit/custom-attacks/tree/main/examples/nodejs-logging-attack)
example.
