# How To Write An Attack Extension

This how-to article will teach you how to write an extension using ActionKit that adds new attack capabilities. We will look closely at existing extensions to learn about semantic conventions, best practices, expected behavior and necessary boilerplate.

The article assumes that you have read the [overview documentation](../action-api.md#overview) for the Action API and possibly skimmed over the expected API endpoints. We are leveraging the Go programming language within the examples, but you can use every other language as long as you adhere to the expected API.

## Necessary Boilerplate

https://github.com/steadybit/action-kit/blob/128d8c05bdadb54e8b001391ead530e22d2d17a3/examples/go-kubectl/main.go#L14-L30