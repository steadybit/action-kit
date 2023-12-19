# Extension Registration

Steadybit's agents need to be told where they can find extensions.

## With Automatic Kubernetes Annotation Discovery

The annotation discovery mechanism is based on the following annotations on service or daemonset level:

``` 
steadybit.com/extension-auto-discovery:                                                                                                                                                                              
  {                                                                                                                                                                                                                
    "extensions": [                                                                                                                                                                                                
      {                                                                                                                                                                                                            
        "port": 8088,                                                                                                                                                                                              
        "types": ["ACTION","DISCOVERY","EVENT"],                                                                                                                                                                           
        "protocol": "http"                                                                                                                                                                                               
      }                                                                                                                                                                                                          
    ]                                                                                                                                                                                                    
  }
```

If you are using our helm charts, the annotations are automatically added to the service or daemonset definitions of the extension.

## With Environment Variables

If you can't use the automatic annotation discovery, for example if you are not deploying to kubernetes, you can still register extensions using
environment variables. You can be specify them via `agent.env` files or directly via the command line.

Please note that these environment variables are index-based (referred to as `n`) to register multiple extension instances.

| Environment Variable<br/>(`n` refers to the index of the extension's instance) | Required | Description                                                                                                                                                                                  |
|--------------------------------------------------------------------------------|----------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_n_URL`                                     | yes      | Fully-qualified URL of the endpoint [listing supported actions](./action-api.md#action-list) of an extension, e.g., `http://my-extension.steadybit-extension.svc.cluster.local:8080/actions` |
| `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_n_METHOD`                                  |          | Optional HTTP method to use. Default: `GET`                                                                                                                                                  |
| `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_n_BASIC_USERNAME`                          |          | Optional basic authentication username to use within HTTP requests.                                                                                                                          |
| `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_n_BASIC_PASSWORD`                          |          | Optional basic authentication password to use within HTTP requests.                                                                                                                          |

### Example
To specify, e.g., the fully-qualified URL of two extensions, where the second one requires basic authentication, you use

- `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_0_URL`,
- `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_1_URL`,
- `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_1_BASIC_USERNAME` and
- `STEADYBIT_AGENT_ACTIONS_EXTENSIONS_1_BASIC_PASSWORD`.


