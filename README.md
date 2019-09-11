# golang-template

This repository contains a basic template for new Nalej golang-based projects.

## New project checklist

1. Setup `.nalej-component.json` file (See [Set the applications](#set-the-applications)).
2. Rename `cmd` application folders to match your application list.
3. (Optional) If using Docker images, rename `components` application folders and K8s resource files.
4. Set the initial component version (See [Set the version](#set-the-version)).
5. Rename application (and image if applicable) list variables on CI pipeline (See [CI integration](#ci-integration)).
6. Setup dependencies (See [Configure dependencies](#configure-dependencies)).

## Guidelines

This repository follows the project structure recommended by the [golang community] (https://github.com/golang-standards/project-layout).
Please refer to the previous URL for more information. We describe some relevant folders:

* `bin`: Binary or distributable files.
* `ci`: CI/CD pipelines and related files.
* `cmd`: Applications contained in this repo. This folder contains the minimal expression of executable applications.
This means that code such as algorithms, servers, database queries MUST not be there.
* `components`: Definition of components, there corresponding Docker images and regarding files if needed. Normally, Docker
images will simply incorporate the main Golang compiled file running an app.
* `pkg`: this folder will contain most of the code supporting the applications.
* `scripts`: relevant scripts needed to run applications/configure them or simply helping.
* `util`: any code that without being really part of an application is useful in some context.

And some relevant files:

* `Gopkg.toml`: dependencies needed to run must be indicated here, specially if a certain version of a repo is required.
* `Makefile`: the main compilation manager for projects. More information below.
* `ci/azure-pipelines.main.yaml`: CI pipeline file. More information below.
* `README.md`: (This file) Description of the project and examples of use.
* `.nalej-component.json`: This file describes the component. It contains the language used, additional tools required and the application list.
* `.version`: One single line indicating the current version of the repo. For example: v0.0.1, v1.2.3, etc.

## Makefile

Use the `make` command to control the generation of executables, management of dependencies, testing and Docker
images. Minor modifications have to be done in order to adapt a project with the structure defined in this document
to be compiled using the current Makefile.

The `Makefile` file on the root does nothing on its own but to import the specific Makefiles contained on the `scripts` directory. To add or remove capabilities to this "root" Makefile, add them in the `.nalej-component.json` file. Once this change hits the master branch, the CI will propagate the required Makefile changes to your component automatically.

## CI integration

Inside the `ci` folder is the main pipeline definition file for the CI called `azure-pipelines.main.yaml`. You only need to change two variables at the start of the file called `appList` and `imageList` to match the applications and Docker images you are creating on your component. Once you have a viable component, ask to get your component added to the CI/CD platform to start getting automatic feedback from every change you push to the repository.

## Set the applications

In order to know what applications have to be generated, the list of applications have to be set. This is a blank
space separated list. The name of the apps must be the same of the folders under the cmd. This is done modifying the `application_list` variable in the `.nalej-component.json` file.

For the current example, example-app and other-app.

```json
"application_list": [
  "example-app other-app"
]
```

## Set the version

Any developed solution will be associated with a version. The value of the version must be indicated in the `.version` file.

## All at once

Running `make all` will execute the most relevant tasks except those regarding Docker management. The resulting files
can be found at the `bin` folder.

For more information about particular aspects of the compilation process, please check the following sections.

## Configure dependencies

We use godep for the management of dependencies. To automatically update your dependencies and download the required
packages into the vendor folder run `make dep`.

## Build the apps

The `make build` command generates executables compatible with your current OS. If you need to customize OS or architecture, please use `make build-custom`, setting the `BUILDOS` and `BUILDARCH` make variables. To get a full list of possible combinations, please execute `go tool dist list` to get it.

Examples:

Building for your OS
```
make build
>>> Building binaries for your OS
 - Built example-app binary
 - Built other-app binary
>>> Finished building binaries for your OS
```

Building for GNU/Linux for AMD64 architecture
```
make build-custom BUILDOS=linux BUILDARCH=amd64
>>> Building binaries for linux amd64
 - Built example-app binary
 - Built other-app binary
>>> Finished building binaries for linux amd64
```

## Testing

Testing runs all the available go tests for the current project. A `make test-all` is available to run just at once
regular, coverage and race testing. Run `make test`, `make test-coverage` and `make test-race` to run regular,
coverage and race tests accordingly. Any outcome is stored under the bin folder.

## Building Docker images

The `make image` option will build all the available Docker images if and only if a Dockerfile is found for the
application in the components folder. For example, in the current example only the Docker image for example-app is
generated. Images are tagged using the version value set in the version file.

## Publishing Docker images

We use Azure Container Registry as our Docker Registry provider. Executing `make publish` will try to log you in Azure before publishing the Docker images.

We have different registries for different purposes:

* `nalejdev`: Development registry. All images are namespaced by user, this is the default registry used by `make publish`.
* `nalejstaging`: Staging registry. This images are considered at least stable and should at least run. All changes in the master branch are pushed to this registry automatically by the CI/CD Platform.
* `nalejregistry`: Production registry. This images are freezed and released versions of every Nalej component. No push are allowed to this registry.
* `nalejpublic`: Public registry. This is a special registry used by images meant to be used in a public environment or user space.

To specify the registry you want to use, you must define the `DOCKER_REGISTRY` make variable with one of the following options:

* `development`: To use `nalejdev` registry. This is the default option if not provided.
* `staging`: To use `nalejstaging` registry.
* `production`: To use `nalejregistry` registry.
* `public`: To use `nalejpublic` registry.

Example:

```
make publish DOCKER_REGISTRY=staging
```

## Working with K8s and Azure Container Registry
To pull images from a private ACR from a Kubernetes cluster requires a different approach than the one we use to publish images. Instead of using your Azure account credentials, we need to use a Service Principal account and set those credentials as a K8s Secret resource on the cluster before pulling any image.

To get the Service Principal account details, please contact with the DevOps team.

The `docker_creds.sh` script found in the `scripts` folder is a helper script to help you create this K8s Secret. You just need to export some environment variables and execute the script to create the Secret.

This variables are the following:

* `DOCKER_REGISTRY_SERVER`: Server URL, for our ACR registries this is made of the registry name (you can found the names on the registry list on the Publishin Docker images section) followed by `.azurecr.io`. For example for the `nalejstaging` registry, the URL is `https://nalejstaging.azurecr.io`.
* `DOCKER_USER`: Service Principal Application ID.
* `DOCKER_PASSWORD`: Service Principal Password.
* `DOCKER_EMAIL`: Your E-Mail address.

Once you have the secret created (the secret name is `nalej-registry` by default), you need to modify the `name` parameter of the `imagePullSecrets` variable list on the K8s Deployment (or DaemonSet, StatefulSet, etc.) to match the secret name.

Example:
```yaml
kind: Deployment
apiVersion: apps/v1
metadata:
  labels:
    app: application
    component: example
  name: example
  namespace: default
spec:
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: application
      component: example
  template:
    metadata:
      labels:
        app: application
        component: example
    spec:
      containers:
      - name: example
        image: nalejstaging.azurecr.io/nalej/example-app:v0.0.1
        imagePullPolicy: Always
        securityContext:
          runAsUser: 2000
      imagePullSecrets:
      - name: nalej-registry
```
