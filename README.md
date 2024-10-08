[![Tests Status](https://github.com/unpackdev/fdb/actions/workflows/test.yml/badge.svg)](https://github.com/unpackdev/fdb/actions/workflows/test.yml)
[![Build Status](https://github.com/unpackdev/fdb/actions/workflows/build.yml/badge.svg)](https://github.com/unpackdev/fdb/actions/workflows/build.yml)
[![Security Status](https://github.com/unpackdev/fdb/actions/workflows/gosec.yml/badge.svg)](https://github.com/unpackdev/fdb/actions/workflows/gosec.yml)
[![Coverage Status](https://coveralls.io/repos/github/unpackdev/fdb/badge.svg?branch=main)](https://coveralls.io/github/unpackdev/fdb?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/unpackdev/fdb)](https://goreportcard.com/report/github.com/unpackdev/fdb)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/unpackdev/fdb)](https://pkg.go.dev/github.com/unpackdev/fdb)
[![Discord](https://img.shields.io/discord/1109929011896909875.svg)](https://discord.gg/PdHVbuTQRX)


# (f)db

**NOTE: At the moment, I am exploring and integrating all potential high-performance transport options, including 
TCP, to establish a solid foundation for benchmarking. In the future, I may discard any transport methods that 
prove inefficient based on the benchmarking results. Concurrently, I will begin writing convenient wrappers around 
the underlying packages to streamline usage and deployments.**

**CURRENTLY UNDER ACTIVE DEVELOPMENT**

**Meaning everything about this repository can be changed. Best to keep track, not use the code yet besides to play with it...**

(f)db is born from frustrations experienced while working with key-value (KV) databases. 
The goal is to build extremely fast transport layers on top of KV databases.

This project is going to be either f**k databases or fast database... There is no third solution...

### Why is this necessary, and is it just another over-engineered solution in the web world?

It might not be essential for every use case, but it becomes particularly interesting if you require a distributed key-value database with replicated data across multiple nodes. 
Additionally, it offers the ability to access and manage data efficiently across multiple servers, each sharing a single connection endpoint, 
to offload data to different databases and/or nodes.

For instance, imagine using the same client to seamlessly push data to either an MDBX or DuckDB instance, 
regardless of whether they are hosted on the same or different servers. 
This abstraction provides flexibility in data management and replication across distributed systems.

### Why I need this?

I require a solution that enables different services to write to and read from different databases while using a unified transport layer. Achieving this with existing tools is complex, if not impossible, without a dedicated transport layer on top of the database. This transport layer allows for cross-service communication and interaction with the underlying database.

This project addresses that gap by building a high-performance, flexible transport layer that enables seamless interaction with various databases in a distributed environment.

### Why Multiple Transports?

One core feature of (f)db is its support for multiple transport protocols, including UDP, UDS, TCP, and QUIC. The reason for this is simple: no single transport protocol is perfect. Each excels in different areas, such as throughput, latency, reliability, or efficiency under specific network conditions.

By offering multiple transport options, (f)db provides the flexibility to fine-tune and optimize performance depending on your specific needs and environment. This allows users to select the most appropriate transport based on their use case, whether it's high-throughput, low-latency, or optimized for specific infrastructure.

### Networking

This project uses [gnet](https://github.com/panjf2000/gnet), an event-driven, high-performance networking library built for Go. gnet is much faster than Go’s standard net package for several reasons:

- Event-Driven Model: gnet uses an event-driven approach, meaning it reacts to specific network events (like data being available to read or connections being closed) instead of continually polling or blocking threads like the standard net package. This reduces CPU overhead and leads to faster processing.
- Efficient Use of Goroutines: Unlike Go’s net package, gnet minimizes the use of goroutines, which helps reduce the context-switching overhead that can slow down highly concurrent applications. Instead, gnet directly handles network I/O in an optimized way.
- Zero-Copy Data Handling: gnet offers zero-copy mechanisms for data processing, meaning that memory is not repeatedly copied between kernel and user space, significantly improving throughput for high-performance networking applications.
- Multi-Event Processing: gnet allows the processing of multiple events within a single loop, which can lead to higher efficiency when handling a large number of simultaneous connections. This feature is particularly valuable for real-time, high-frequency applications that need to handle many clients at once.

By leveraging gnet’s capabilities, (f)db can react quickly to network events, optimize throughput, and minimize latencies, making it ideal for distributed systems where performance is critical.


### eBPF Integration

As part of the ongoing development of high-performance transport layers for (f)db, eBPF (Extended Berkeley Packet Filter) has been integrated to enhance packet processing and monitoring capabilities within the network stack.

By leveraging eBPF, we can build efficient, scalable, and secure transport layers that directly interact with network traffic, providing real-time insights and optimizations for packet flow, without adding latency or reducing throughput.

- Real-Time Packet Filtering: With eBPF, (f)db can selectively filter out unnecessary traffic at the kernel level, significantly reducing the amount of data that needs to be processed in user space.
- Network Performance Insights: eBPF programs can monitor performance metrics, such as packet drops, latencies, and bandwidth usage, allowing for dynamic tuning of transport protocols based on real-time traffic conditions.
- Low-Latency Processing: eBPF’s ability to operate within the kernel ensures that packet processing happens as close to the hardware as possible, minimizing delays and improving overall system responsiveness.
- Ring Buffer for Data Handling: eBPF uses a ring buffer to store and transfer network data efficiently to user-space applications, ensuring fast and reliable communication between the kernel and (f)db.

**Right now some basic program exists that can be loaded but it does not do anything more then writing into ring buffer**

This is something to be dealt with later on...

For example, can be used for mass writes without ACK, different types of discoveries or for example DDoS detection or non-whitelisted server ip access, idk...

### Future Considerations

While achieving all these goals without using advanced techniques like DPDK (Data Plane Development Kit) will be challenging, the initial prototype will focus on building a solid foundation. Advanced optimizations can be layered on top later, depending on the project's needs and performance demands.

This approach ensures that (f)db remains both adaptable and scalable, with the potential to handle a variety of use cases while maintaining high performance.


## Diagram

```mermaid
graph TD;
    %% Main Entry Point and CLI Commands
    A[Main Entry Point - main.go] --> B[CLI Manager - urfave/cli]
    B --> C1[Certs Command]
    B --> C2[Benchmark Command]
    B --> C3[Serve Command]

    %% FDB Initialization
    C2 -->|Initializes| D[FDB Instance]
    C3 -->|Initializes| D

    %% FDB Components
    D -->|Creates| E[Transport Manager]
    D -->|Creates| F[Database Manager]
    D -->|Sets up| G[Logger]
    D -->|Configures| H[Pprof Profiler]

    %% Transport Manager and Transports
    E -->|Registers| I1[QUIC Transport]
    E -->|Registers| I2[UDS Transport]
    E -->|Registers| I3[TCP Transport]
    E -->|Registers| I4[UDP Transport]
    E -->|Registers| I5[Dummy Transport]

    %% Individual Transports and Servers
    I1 -->|Implements| J1[QUIC Server]
    I2 -->|Implements| J2[UDS Server]
    I3 -->|Implements| J3[TCP Server]
    I4 -->|Implements| J4[UDP Server]
    I5 -->|Implements| J5[Dummy Server]

    %% Handlers and Operations
    subgraph Transport Handlers
        J1 --> K[Read/Write Handlers]
        J2 --> K
        J3 --> K
        J4 --> K
        J5 --> K
    end

    K -->|Performs| L1[Handle Read]
    K -->|Performs| L2[Handle Write]

    %% Database Interaction
    L1 --> M[Database Manager]
    L2 --> M

    M -->|Uses| N[MDBX Database]

    %% Benchmarking Suite
    subgraph Benchmarking
        C2 --> O[Suite Manager]
        O -->|Manages| P1[QUIC Suite]
        O -->|Manages| P2[Dummy Suite]
        O -->|Manages| P3[UDS Suite]
        O -->|Manages| P4[TCP Suite]
        O -->|Manages| P5[UDP Suite]

        P1 -->|Runs| Q[Write/Read Benchmarks]
        P2 -->|Runs| Q
        P3 -->|Runs| Q
        P4 -->|Runs| Q
        P5 -->|Runs| Q
    end

    %% Configuration and Logger
    D --> R[Configuration]
    D --> G
    G --> S[Zap Logger]

    %% Pprof Profiler
    D --> H
    H --> T[Pprof Server]

    %% Database Manager Details
    M --> U[DB Manager]
    U --> V[Provides DB Access]

    %% Legend
    classDef component fill:#fcfcfc,stroke:#333,stroke-width:2px;
    class A,B,C1,C2,C3,D,E,F,G,H,M,N,O,P1,P2,P3,P4,P5,Q,R,S,T,U,V component;
```


### Explanation of the Diagram:

1. **Main Entry Point**: This is the main starting point of the application, located in `main.go`. It utilizes the `urfave/cli` package to manage multiple CLI commands, such as `certs`, `benchmark`, and `serve`. Each command has specific functionality related to certificate management, benchmarking, or starting the server.

2. **FDB Initialization**: Both the `benchmark` and `serve` commands initialize an `FDB Instance`. This instance manages core components such as the transport manager, database manager, logger, and performance profiler (`pprof`).

3. **Transport Manager**: The transport manager handles the registration of various transport types. These transports include `QUIC`, `UDS`, `TCP`, `UDP`, and `Dummy` transport types, each having its own corresponding server implementation.

4. **Transports and Servers**: Each registered transport is associated with a specific server. For example, `QUIC` uses a `QUIC Server`, `UDS` uses a `UDS Server`, and so on. Each server has a set of read and write handlers for processing incoming and outgoing messages.

5. **Handlers and Operations**: The read and write handlers are responsible for processing messages. They handle both reading from and writing to the underlying transport and interact with the database manager to store and retrieve data.

6. **Database Manager**: The database manager interacts with the `MDBX` database, providing access to key-value storage. Both read and write handlers utilize the database manager to perform database operations.

7. **Benchmarking Suite**: The `benchmark` command initializes a `Suite Manager` which manages multiple benchmarking suites for each transport type, such as `QUIC Suite`, `Dummy Suite`, `UDS Suite`, `TCP Suite`, and `UDP Suite`. Each suite runs a set of benchmarks that measure the read/write performance of the corresponding transport.

8. **Configuration and Logger**: The FDB instance loads configuration details and initializes a `Zap Logger` for structured logging. Configuration files are used to manage settings for transports, database, and other components.

9. **Pprof Profiler**: The `pprof` profiler is used to monitor the performance of the application. It is set up within the FDB instance and exposes a profiling server for detailed analysis of memory and CPU usage during runtime.

10. **Database Interaction**: During both read and write operations, the database manager interacts with the underlying `MDBX` database to store and retrieve data, ensuring efficient key-value storage management.

11. **CLI Commands**:
- **Certs Command**: Manages certificate generation and handling.
- **Benchmark Command**: Runs benchmarking tests for each transport, collecting performance metrics like throughput and latency.
- **Serve Command**: Starts the FDB server with all configured transports.

12. **Legend**: The diagram outlines the key components and their relationships, showing how the transport manager interacts with the servers, handlers, and database, along with the benchmarking and profiling systems.


## Usage

### As a Library

To include `fdb` in your project, use the following command to install the package:

```bash
go get github.com/unpackdev/fdb
```

Make sure to review the documentation for proper integration and usage within your Go application.

### Docker

To run the fdb instance in a production-like environment, along with supporting services like OpenTelemetry and Jaeger for tracing and monitoring, follow these steps:
- Ensure you have Docker and Docker Compose installed.
- Clone the repository and start the services using Docker Compose.

```bash
git clone https://github.com/unpackdev/fdb
cd fdb
docker-compose up -d
```

This will bring up the [(f)db](https://github.com/unpackdev/fdb) instance, [OpenTelemetry](https://opentelemetry.io/docs/languages/go/) collector, and [Jaeger](https://www.jaegertracing.io/) for tracing, making the system ready for production-level monitoring and telemetry.


### Loading the eBPF Program

To get started, you'll first need to download and update your local environment by installing the necessary dependencies. 
Additionally, ensure that you have the (f)db project built to properly load and unload the eBPF program.

```
make deps
make build
sudo setcap cap_bpf+ep c/obj/ebpf_program.o
```

Next, compile the eBPF program located at [program](./c/src/ebpf_program.c).

```
make ebpf-build
```

Before loading the program, identify the network interface you want to bind the eBPF program to. 
The default interface is typically eth0, but it can vary based on your system configuration. 
To inspect your available network interfaces, use the following command:

```
./build/fdb ebpf interfaces
```

Finally, load the eBPF program onto your desired network interface:

```
sudo make ebpf-load INTERFACE={interface-name}
```

Once you’ve completed the above steps, your output should look something like this:

```
xxx:xx$ make ebpf-build && sudo make ebpf-load INTERFACE=xxx
eBPF program compiled successfully.
sudo ip link set dev enp73s0 xdp obj c/obj/ebpf_program.o sec xdp
eBPF program loaded onto interface xxx.
```

With that, your eBPF program is successfully loaded and running on the specified interface!

### Running the Binary

For a more custom or direct deployment, you can build and run the fdb binary manually.

- 1: Clone the repository

```bash
git clone https://github.com/unpackdev/fdb
cd fdb
```

- 2: Modify the config.yaml file as per your environment's requirements.
- 3: Build and start the server

```bash
make build
./build/fdb serve
```

Ensure that your configuration file is tuned for production, including settings for transports, database paths, logging levels, and performance profiling.
By default, this will start the server with all the transports and services configured in the config.yaml, ready for high-performance and production use.

## QUIC (HTTP/3)

https://github.com/quic-go/quic-go/wiki/UDP-Buffer-Sizes




```
sysctl -w net.core.rmem_max=7500000
sysctl -w net.core.wmem_max=7500000
```

## TODO

- Due to changes in the entire logic now unit tests are broken. 

## IDEAS

- P2P Sync... (Supervisors vs. Readers a.k.a. validators vs clients)
- Could be grpc sync as well... Need to see complexity vs. benefits...

## Commands

### Certificates and co.

```
make build && ./build/fdb certs --cert=./data/certs/cert.pem --key=./data/certs/key.pem
```

### Benchmark

```
make build && ./build/fdb benchmark --suite quic --clients 5 --messages 1000 --type write
```

## Benchmarks

There is a dummy transport, starts the (gnet) UDP and does pretty much nothing. We're going to 
use that one as a baseline for any other benchmark.

### DUMMY

#### Write Benchmark

```
make build && ./build/fdb benchmark --suite dummy --clients 50 --messages 1000000 --type write --timeout 120

--- Benchmark Report ---
Total Clients: 50
Messages per Client: 1000000
Total Messages: 50000000
Success Messages: 50000000
Failed Messages: 0
Total Duration: 13.604984597s
Average Latency: 10.925µs
P50 Latency: 5.87µs
P90 Latency: 7.4µs
P99 Latency: 14.56µs
Throughput: 3,675,123 messages/second
Memory Used: 6.05 MB
Latency Jitter (StdDev): 346.418350µs

```

### TCP

#### Write Benchmark

```
make build && ./build/fdb benchmark --suite tcp --clients 50 --messages 200000 --type write --timeout 120

--- Benchmark Report ---
Total Clients: 50
Messages per Client: 200000
Total Messages: 10000000
Success Messages: 10000000
Failed Messages: 0
Total Duration: 17.935868899s
Average Latency: 83.1µs
P50 Latency: 64.572µs
P90 Latency: 122.153µs
P99 Latency: 304.218µs
Throughput: 557,541 messages/second
Memory Used: 667.91 MB
Latency Jitter (StdDev): 148.417551µs
```

### QUIC

#### Write Benchmark

```
make build && ./build/fdb benchmark --suite quic --clients 50 --messages 100000 --type write --timeout 120

--- Benchmark Report ---
Total Clients: 50
Messages per Client: 100000
Total Messages: 5000000
Success Messages: 5000000
Failed Messages: 0
Total Duration: 54.655111416s
Average Latency: 543.478µs
P50 Latency: 521.064µs
P90 Latency: 945.644µs
P99 Latency: 1.603621ms
Throughput: 91,482 messages/second
Memory Used: 17260.96 MB
Latency Jitter (StdDev): 319.379812µs
```

### UDP

#### Write Benchmark

```
make build && ./build/fdb benchmark --suite udp --clients 50 --messages 100000 --type write --timeout 120

--- Benchmark Report ---
Total Clients: 50
Messages per Client: 100000
Total Messages: 5000000
Success Messages: 5000000
Failed Messages: 0
Total Duration: 16.771189289s
Average Latency: 169.167µs
P50 Latency: 128.563µs
P90 Latency: 307.689µs
P99 Latency: 877.784µs
Throughput: 298,130 messages/second
Memory Used: 678.49 MB
Latency Jitter (StdDev): 173.144187µs
```

## LICENSE

For more details about this license, please refer to the [LICENSE](LICENSE) file included in this repository.
