# How To Write An Extension That Queries Metrics

This how-to article will teach you how to write an extension using ActionKit that adds a new action that queries metrics from Prometheus. We will look closely at existing extensions to learn about semantic conventions, best practices, expected behavior and necessary boilerplate.

The article assumes you have authored an action before or read the [How To Write An Attack Extension article](./write-an-attack-extension.md). We are leveraging the Go programming language within the examples, but you can use every other language if you adhere to the expected API.

<!-- TOC -->
* [How To Write An Extension That Queries Metrics](#how-to-write-an-extension-that-queries-metrics)
  * [Overview from an End-User Perspective](#overview-from-an-end-user-perspective)
  * [Overview from an Extension Developer Perspective](#overview-from-an-extension-developer-perspective)
  * [Extending the Action Description](#extending-the-action-description)
  * [Overview](#overview)
  * [Handling Query Requests](#handling-query-requests)
<!-- TOC -->

## Overview from an End-User Perspective

Before we dive into the implementation, let us first understand what metric query support in ActionKit means for end-users. We will explain this using our Prometheus extension.

Any action with metric query support gains a new section within the action configuration sidebar. Through this section, users can define zero or more metric queries that will be sent to the extension for as long as this action runs within the experiment.

![An image showing the Prometheus action's help text about metric queries.](./img/prom-01-empty.png)

The image above shows that the Prometheus action gains a new configuration element for metric queries. It is important to note that the definition of metric queries is up to the extension. For Prometheus, a metric query is a PromQL query. Other systems may have other query systems, e.g., Elasticsearch has JSON objects for its request body search and the Lucene query language. ActionKit doesn't make any assumptions about the query parameters, as you will learn in this how-to.

![The image depicts the metric query definition popup for Prometheus.](./img/prom-02-define-query.png)

A popup asks for a label and the parameters required for the action's metric queries. As mentioned before, this is a PromQL query for Prometheus.

![The Steadybit UI know lists the created metric query and invites the user to define a metric check](./img/prom-03-query-defined.png)

Once at least one metric query is defined, the user unlocks the ability to define metric checks. Metric checks can be used to fail experiments when certain conditions are met, e.g., metric values too high/low or when queries have results. The latter exists to support alerting patterns similar to how Prometheus alerts function.

![A metric check is defined failing the experiment whenever the metric value is greater than or equal to 42](./img/prom-04-check-defined.png)

This action will fail the experiment whenever the metric value for `HighRequestLatency` is greater than or equal to 42 – for any data series received via the metric query. Furthermore, users may also choose to compare two metrics against each other, as the following image will show.

![Two metrics are compared against each other.](./img/prom-05-cross-metric-query.png)

The image above shows a typical check within Kubernetes environments: Ensure all pods are ready. You can imagine how powerful and valuable this mechanism can be.

At last, the metric values are also available within the experiment execution view. Metric values are plotted over time, including their labels.

![Metric values are plotted within the experiment execution view over time.](./img/prom-06-experiment-execution-view.png)

## Overview from an Extension Developer Perspective

## Extending the Action Description

```go
// Source: https://github.com/steadybit/extension-prometheus/blob/66687c2ab745d22c0c2cb5b258f6c51b13d8e0a3/extmetric/extmetric.go#L60

import (
    "github.com/steadybit/action-kit/go/action_kit_api/v2"
    "github.com/steadybit/extension-kit/extutil"
)

action_kit_api.ActionDescription{
    // other field removed for brevity
    Metrics: extutil.Ptr(action_kit_api.MetricsConfiguration{
        Query: extutil.Ptr(action_kit_api.MetricsQueryConfiguration{
            Endpoint: action_kit_api.MutatingEndpointReferenceWithCallInterval{
                Method:       action_kit_api.Post,
                Path:         "/prometheus/metrics/query",
                CallInterval: extutil.Ptr("1s"),
            },
            Parameters: []action_kit_api.ActionParameter{
                {
                    Name:     "query",
                    Label:    "PromQL Query",
                    Required: extutil.Ptr(true),
                    Type:     action_kit_api.String,
                },
            },
        }),
    }),
}
```

## Overview



## Handling Query Requests

```go
// Source https://github.com/steadybit/extension-prometheus/blob/66687c2ab745d22c0c2cb5b258f6c51b13d8e0a3/extmetric/extmetric.go#L95
```