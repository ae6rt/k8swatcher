## Kubernetes Pod Watcher

This simple tool shows how to use a client websocket to monitor
Kubernetes pod events.  Such a tool could be useds as the basis for
watcher utility that registers pods in an external service registration
system.

### Build

```
go build
```

### Run

```
$ ./k8swatcher  -username user -password sekrit -ca-cert-file ca.crt -addr kubernetes:6443
```

which produces this typical output

```
recv: {"type":"ADDED","object":{"kind":"Pod","apiVersion":"v1","metadata":...}}
```

