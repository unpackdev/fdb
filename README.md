# (f)db

**NOTE: At this moment I am adding all possible faster ways, including TCP, to be able to do proper benchmarking first.
In the future, I may drop transports that prove to be inefficient based on benchmark results. At the same time I will slowly start to write wrappers around the packages for convenient usage incl. deployments.**

This is currently a prototype, with the idea of building incredibly fast transport layers on 
top of key-value (KV) databases. The goal is to allow one or multiple instances of these 
databases to be started and cross-shared in user space or accessed remotely bypassing general locks
that are enforced in KV databases or some of the OLAP databases such as DuckDB.

We're aiming to create a database wrappers so fast and efficient that it could be called the 
"f**k database" — a no-nonsense, high-performance solution for extreme speed and scalability suitable for HFS (High Frequency Trading) or basically
as a troll and actual description as this task is quite hard to achieve.

Though this will be hard to achieve without DPDK. Will not overkill the prototype with it for now...



## GNET

gnet is a high-performance, lightweight, non-blocking, event-driven networking framework written in pure Go.

gnet is an event-driven networking framework that is ultra-fast and lightweight. It is built from scratch by exploiting epoll and kqueue and it can achieve much higher performance with lower memory consumption than Go net in many specific scenarios.

https://github.com/panjf2000/gnet



## QUIC (HTTP/3)

https://github.com/quic-go/quic-go/wiki/UDP-Buffer-Sizes

```
sysctl -w net.core.rmem_max=7500000
sysctl -w net.core.wmem_max=7500000
```

## IDEAS

- P2P Sync... (Supervisors vs. Readers a.k.a. validators vs clients)
- Could be grpc sync as well... Need to see complexity vs. benefits...

## Unit Tests

```
go test -v -cover
=== RUN   TestManagerDbOperations
=== RUN   TestManagerDbOperations/Set_and_Get_Key
=== RUN   TestManagerDbOperations/Check_Exists_Key
=== RUN   TestManagerDbOperations/Delete_Key
--- PASS: TestManagerDbOperations (0.02s)
    --- PASS: TestManagerDbOperations/Set_and_Get_Key (0.01s)
    --- PASS: TestManagerDbOperations/Check_Exists_Key (0.01s)
    --- PASS: TestManagerDbOperations/Delete_Key (0.01s)
=== RUN   TestUDPServer
2024/09/14 15:44:07 Awaiting for started closure...
2024/09/14 15:44:07 UDP Server started on udp://127.0.0.1:8781
2024/09/14 15:44:07 UDP Server is listening on 127.0.0.1:8781
2024/09/14 15:44:07 Closed started...
2024/09/14 15:44:07 Started closure detected...
=== RUN   TestUDPServer/Valid_Write_and_Read
    udp_server_test.go:172: Response from server after write: Message written to database
    udp_server_test.go:214: Response from server after read: test value
=== RUN   TestUDPServer/Invalid_Key_Length_(Too_Short)
2024/09/14 15:44:08 Invalid message length: 32, expected at least 34 bytes
    udp_server_test.go:172: Response from server after write: Invalid message format
    udp_server_test.go:179: Received expected error response: Invalid message format
=== RUN   TestUDPServer/Invalid_Handler_Type
    udp_server_test.go:172: Response from server after write: ERROR: Invalid action
    udp_server_test.go:179: Received expected error response: ERROR: Invalid action
=== RUN   TestUDPServer/Empty_Data
2024/09/14 15:44:08 Invalid message length: 33, expected at least 34 bytes
    udp_server_test.go:172: Response from server after write: Invalid message format
    udp_server_test.go:179: Received expected error response: Invalid message format
--- PASS: TestUDPServer (0.10s)
    --- PASS: TestUDPServer/Valid_Write_and_Read (0.09s)
    --- PASS: TestUDPServer/Invalid_Key_Length_(Too_Short) (0.00s)
    --- PASS: TestUDPServer/Invalid_Handler_Type (0.00s)
    --- PASS: TestUDPServer/Empty_Data (0.00s)
PASS
coverage: 66.3% of statements
ok  	github.com/unpackdev/fdb	0.132s
```


## Benchmarks

```
go test -run=^$ -bench=BenchmarkUDPServerWrite -v
goos: linux
goarch: amd64
pkg: github.com/unpackdev/fdb
cpu: AMD Ryzen Threadripper 3960X 24-Core Processor 
BenchmarkUDPServerWrite
BenchmarkUDPServerWrite-48    	  279058	      4473 ns/op
PASS
ok  	github.com/unpackdev/fdb	2.879s
```