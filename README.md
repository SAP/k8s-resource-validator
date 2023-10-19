[![REUSE status](https://api.reuse.software/badge/github.com/SAP/k8s-resource-validator)](https://api.reuse.software/info/github.com/SAP/k8s-resource-validator)

# k8s-resource-validator

## Description
A Go library that retrieves resources from a Kubernetes cluster and verifies their validity. Invalid resources are logged as such.

**Note:** If a malicious actor gains write access to your cluster's API, they could change the `k8s-resource-validator`'s settings, or even prevent the validator from running. [Keep your cluster secure](https://kubernetes.io/docs/tasks/administer-cluster/securing-a-cluster/)!

## Requirements
This repo is a [Go](https://go.dev/doc/install) library that is meant to be imported by other Go applications.

For detailed information about setup and usage see [here](docs/DETAILS.md).

## Contributing
This project is open to feature requests/suggestions, bug reports etc. via [GitHub issues](https://github.com/SAP/k8s-resource-validator/issues). Contribution and feedback are encouraged and always welcome. For more information about how to contribute, the project structure, as well as additional contribution information, see our [Contribution Guidelines](CONTRIBUTING.md).

## Code of Conduct
We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/SAP/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## Licensing
Copyright 2023 SAP SE or an SAP affiliate company and k8s-resource-validator contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/SAP/k8s-resource-validator).

## Security / Disclosure
If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/SAP/k8s-resource-validator/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.
