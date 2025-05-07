# FDB TODO List

## Core Production Features

### Record Versioning and Conflict Resolution

- Add version tracking to the `Record` structure:
  ```go
  // Record represents a key-value pair in a record batch
  type Record struct {
      Key       [32]byte // Fixed-size byte array for keys
      Value     []byte   // Value as byte slice
      Version   uint64   // Monotonically increasing version number
      Priority  uint8    // Priority level for processing
      Timestamp int64    // Creation/modification timestamp
  }
  ```

- Implement conflict detection and resolution mechanisms:
  - Last-write-wins strategy based on version numbers
  - Optimistic concurrency control for client operations
  - Support for conditional writes based on expected versions

- Update serialization/deserialization methods to handle new fields

### Missing Production-Ready Features

1. **Consensus Protocol**
   - Implement strong consensus for write ordering
   - Consider Raft or PBFT for distributed agreement

2. **Transaction Management**
   - Implement ACID transaction support
   - Add distributed commit/rollback protocols
   - Support for multi-key atomic operations

3. **Failure Detection and Recovery**
   - Automated node failure detection
   - Efficient state transfer for recovering nodes
   - Automatic recovery procedures

4. **State Transfer Optimization**
   - Efficient incremental state transfer
   - Snapshot-based recovery for new/rejoining nodes

5. **Comprehensive Security**
   - Data encryption (at-rest and in-transit)
   - Enhanced authentication mechanisms
   - Audit logging for all operations

6. **Backup & Disaster Recovery**
   - Point-in-time recovery
   - Incremental backup support
   - Disaster recovery procedures

## Performance Optimizations

- Further optimize BatchWriter for higher throughput
- Improve worker selection algorithm for P2P distribution
- Enhance TCP streaming for large payloads
