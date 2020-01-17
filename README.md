# provisioner

This repository contains the `provisioner` component in charge of creating new clusters by using a given infrastructure provider.

## Getting Started

The component includes two binaries, the `provisioner` itself and a CLI called `provisioner-cli` to enable several functionalities that will be explained later. 

### Prerequisites

No dependencies with other Nalej microservices.

### Build and compile

In order to build and compile this repository use the provided Makefile:

```
make all
```

This operation generates the binaries for this repo, download dependencies,
run existing tests and generate ready-to-deploy Kubernetes files.

### Run tests

Tests are executed using Ginkgo. To run all the available tests:

```
make test
```

### Update dependencies

Dependencies are managed using Godep. For an automatic dependencies download use:

```
make dep
```

In order to have all dependencies up-to-date run:

```
dep ensure -update -v
```

## User client interface
To get the certificate status on a provisioned cluster:

```
provisioner-cli cert status [kubeConfigPath] [flags]
```

To provision a new cluster:
```
provisioner-cli provision [flags]
```

To decommission an existing cluster:
```shell script
provisioner-cli decommission --azureCredentialsPath {{path-to-azure-credentials}} --name {{cluster-name}} --platform AZURE --resourceGroup {{resource-group}}
```

## Contributing

Please read [contributing.md](contributing.md) for details on our code of conduct, and the process for submitting pull requests to us.


## Versioning

We use [SemVer](http://semver.org/) for versioning. For the versions available, see the [tags on this repository](https://github.com/nalej/provisioner/tags). 

## Authors

See also the list of [contributors](https://github.com/nalej/provisioner/contributors) who participated in this project.

## License
This project is licensed under the Apache 2.0 License - see the [LICENSE-2.0.txt](LICENSE-2.0.txt) file for details.
