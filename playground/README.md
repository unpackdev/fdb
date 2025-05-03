# FDB Playground

The playground provides a flexible environment for testing and benchmarking the FDB P2P network under various conditions. It allows you to spawn multiple nodes with different roles and configurations, run predefined strategies against them, and monitor their behavior through observability tools like Grafana and OpenTelemetry.

## Features

- Spawn multiple P2P nodes with customizable roles and configurations
- Monitor network latencies, throughput, and node availability
- Test RPC communications between nodes
- Evaluate raw TCP performance and CapnProto serialization
- Benchmark write operations with configurable concurrency and data sizes
- Track per-node metrics with enhanced observability
- Automatic clean shutdown when tests complete

## Usage

### Basic Command Structure

```bash
# General syntax
fdb playground [options] <strategy> [strategy-specific-options]

# Examples
fdb playground --nodes 5 --base-port 9000 write --workers 20 --data-size 100 --total-ops 5000
```

### Global Options

- `--nodes <number>`: Number of nodes to create (default: 3)
- `--base-port <port>`: Starting port number for node communication (default: 8000)
- `--strategy <name>`: Name of the strategy to run (can also be specified as a command)
- `--log-level <level>`: Logging level (debug, info, warn, error)

### Available Strategies

#### Write Strategy

The write strategy tests write operations to the P2P network with configurable parameters:

```bash
fdb playground write [options]
```

Options:
- `--target-node <index>`: Index of the node to send writes to (default: 0)
- `--workers <number>`: Number of concurrent workers (default: 10)
- `--data-size <kb>`: Size of data to write in KB (default: 10)
- `--ops-per-sec <number>`: Target operations per second (default: 100)
- `--total-ops <number>`: Total number of operations (default: 1000)

## Observability

Each node in the playground is assigned a unique identifier (e.g., `playground_peer_0`) that is included in:

1. All log entries
2. Metrics reported to Prometheus
3. Traces exported to OpenTelemetry

This allows for detailed per-node analysis in Grafana dashboards and other observability tools.

## Examples

### Run a Write Test with 5 Nodes

```bash
# Create 5 nodes and run a write test with 20 workers, 100KB data size, and 5000 total operations
fdb playground --nodes 5 write --workers 20 --data-size 100 --total-ops 5000
```

### Custom Port Configuration

```bash
# Start with base port 9000 instead of default 8000
fdb playground --base-port 9000 write
```

### Detailed Logging

```bash
# Enable debug-level logging
fdb playground --log-level debug write
```