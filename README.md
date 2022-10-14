# k8shell
A webshell tool for K8s.

## Build

```bash
cd k8shell
./build.sh
```
if the build is successful, you will get a dist directory in k8shell
```bash
dist
├── frontend
├── kubeconfig
└── main
```
then put your kubeconfig files into the kubeconfig directory in the dist

## Usage

start the webshell server

```
cd k8shell/dist
./main
```

### webshell
`https://{host_ip}:8090/terminal?cluster={kubeconfig-file-name}&namespace={namespace}&pod={pod-name}&container={container-name}cmd={cmd}`

`cmd={cmd}` is optional

For example:

*assume that a kubeconfig file named `abc` already exists in dist/kubeconfig*

`http://127.0.0.1:8090/terminal?cluster=abc&namespace=default&pod=nginx-0&container=nginx`

`http://127.0.0.1:8090/terminal?cluster=abc&namespace=default&pod=nginx-0&container=nginx&cmd=/bin/bash`

### logs
`https://{host_ip}:8090/terminal?cluster={kubeconfig-file-name}&namespace={namespace}&pod={pod-name}&container={container-name}&tail={tailLines}&follow={true or false}`

`tail={tailLines} and follow={true or false}` are optional

For example:

*assume that a kubeconfig file named `abc` already exists in dist/kubeconfig*

`http://127.0.0.1:8090/logs?cluster=abc&namespace=default&pod=nginx-0&container=nginx`

`http://127.0.0.1:8090/logs?cluster=abc&namespace=default&pod=nginx-0&container=nginx&tail=200&follow=true`

Refs:
1. http://maoqide.live/post/cloud/kubernetes-webshell/
2. https://github.com/maoqide/kubeutil