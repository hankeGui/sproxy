# sproxy


## requirement

go 1.13

## build

- server

```shell
go build cmd/server/server.go
```


- client
```shell
go build cmd/client/client.go
```


## example


- server

```shell 
./server -l 0.0.0.0:7443
```

- client
```shell
./client -r <server_ip>:7443
```