# Extension Registration

Steadybit's agents need to be told where they can find extensions. Currently, this is done through environment variables for the Steadybit agent process,
e.g., via `agent.env` with the [Steadybit agent helm chart](https://github.com/steadybit/helm-charts/tree/main/charts/steadybit-agent). The environment
variables are:

- `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_0_URL`: Required fully-qualified URL defining which HTTP URL should be requested to get
  the [list of actions](./action-api.md#action-list), e.g., `http://my-extension.steadybit-extension.svc.cluster.local:8080/actions`.
- `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_0_METHOD`: Optional HTTP method to use. Defaults to GET.
- `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_0_BASIC_USERNAME`: Optional basic authentication username to use within HTTP requests.
- `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_0_BASIC_PASSWORD`: Optional basic authentication password to use within HTTP requests.

These environment variables can occur multiple times with different indices to register multiple extensions,
e.g., `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_0_URL` and `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_1_URL`