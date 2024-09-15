[![Tests Status](https://github.com/unpackdev/fdb/actions/workflows/test.yml/badge.svg)](https://github.com/unpackdev/fdb/actions/workflows/test.yml)
[![Build Status](https://github.com/unpackdev/fdb/actions/workflows/build.yml/badge.svg)](https://github.com/unpackdev/fdb/actions/workflows/build.yml)
[![Security Status](https://github.com/unpackdev/fdb/actions/workflows/gosec.yml/badge.svg)](https://github.com/unpackdev/fdb/actions/workflows/gosec.yml)
[![Coverage Status](https://coveralls.io/repos/github/unpackdev/fdb/badge.svg?branch=main)](https://coveralls.io/github/unpackdev/fdb?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/unpackdev/fdb)](https://goreportcard.com/report/github.com/unpackdev/fdb)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/unpackdev/fdb)](https://pkg.go.dev/github.com/unpackdev/fdb)
[![Discord](https://img.shields.io/discord/1109929011896909875.svg)](https://discord.gg/PdHVbuTQRX)


# (f)db

**NOTE: At this moment I am adding all possible faster ways, including TCP, to be able to do proper benchmarking first.
In the future, I may drop transports that prove to be inefficient based on benchmark results. At the same time I will slowly start to write wrappers around the packages for convenient usage incl. deployments.**

This is currently a prototype, with the idea of building incredibly fast transport layers on 
top of key-value (KV) databases. The goal is to allow one or multiple instances of these 
databases to be started and cross-shared in user space or accessed remotely bypassing general locks
that are enforced in KV databases or some of the OLAP databases such as DuckDB.

This project is going to be either f**k databases or fast database... There is no third solution...

Though this will be hard to achieve without DPDK. Will not overkill the prototype with it for now...

## Diagrams

```mermaid
graph TD;
    A[Main Entry Point - main.go] --> B[CLI Manager - urfave/cli]
    B --> C1[Test Command - Benchmark Client]
    B --> C2[Other Commands - TBD]
    
    C1 -->|Executes Test Command| D1[Client Operations]
    C1 -->|Collects Benchmark Data| D2[Memory Usage]
    C1 -->|Collects Benchmark Data| D3[Execution Time]
    
    subgraph gRPC/QUIC/UDP/UDS Servers
        E1[UDP Server] --> F1[Handler Registry - UDP]
        E2[QUIC Server] --> F2[Handler Registry - QUIC]
        E3[UDS Server] --> F3[Handler Registry - UDS]
    end
    
    F1 --> |Handle Write| G1[WriteHandler]
    F1 --> |Handle Read| G2[ReadHandler]
    
    F2 --> |Handle Write| G1
    F2 --> |Handle Read| G2
    
    F3 --> |Handle Write| G1
    F3 --> |Handle Read| G2
    
    subgraph MDBX Database
        G1 --> H1[Set Key-Value Pair]
        G2 --> H2[Get Key-Value Pair]
    end
    
    subgraph Connection Handling
        D1 -->|Client Operations| I[Connection Handler]
        I -->|Gnet/QUIC| J1[Process Incoming Stream/Frame]
        I -->|Gnet/QUIC| J2[React to Incoming Data]
    end
    
    I --> E1
    I --> E2
    I --> E3
```


### Explanation of the Diagram:

1. **Main Entry Point**: This is where the `main.go` resides. The CLI manager (`urfave/cli`) manages various commands.
2. **Test Command**: This command benchmarks the real client. It includes operations such as client actions and collects metrics like memory usage and execution time.
3. **gRPC/QUIC/UDP/UDS Servers**: These are different servers supported by `fdb`, each with its own handler registry for processing requests.
4. **Handlers**: Each server has a `WriteHandler` and `ReadHandler` that interact with the `MDBX` database to set and get key-value pairs.
5. **Connection Handling**: This is where the incoming connections are processed. It uses `gnet` and `QUIC` to handle streams or frames and react to incoming data.


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

## Commands

### Certificates and co.

```
make build && ./build/fdb certs --cert-output=./data/certs/cert.pem --key-output=./data/certs/key.pem
```

### Benchmark

```
make build && ./build/fdb benchmark --suite-type quic --clients=1 --messages=1000
```

## Benchmarks

### QUIC

```
make build && ./build/fdb benchmark --suite-type quic --clients=1 --messages=1000
Starting benchmark...
2024/09/15 20:55:18 QUIC Server started on 127.0.0.1:4433
QUIC server started successfully

--- Benchmark Report ---
Total Clients: 0
Total Messages: 1000
Success Messages: 1000
Failed Messages: 0
Total Duration: 3.135008519s
Average Latency: 3.130208ms
Throughput: 318.98 messages/second
Memory Used: 0 bytes
QUIC server stopped successfully
```

^ This piece of shit is slow as you can see but at least came to the point where I can start doing optimizations.

## For Developers

- Main entrypoint to the application can be found at [entrypoint](./entrypoint)

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