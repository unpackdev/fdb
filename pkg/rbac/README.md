# RBAC Package

The `rbac` package provides Role-Based Access Control (RBAC) functionalities to manage user roles and permissions within the system. It ensures that users can perform only the actions they are authorized for, enhancing the security and integrity of the application.

## **Features**

- **Role Definitions**: Define various roles such as Admin, Validator, Sequencer, Node, Observer, and User.
- **Permission Management**: Assign specific permissions to each role.
- **User Management**: Create users, assign roles, and manage permissions dynamically.
- **Concurrency Safety**: Thread-safe operations to handle concurrent access in multi-goroutine environments.

## **Roles and Permissions**

### **Roles**

- **Admin**: Full access to manage keys, propose and approve blocks, finalize blocks, manage shards, monitor network, update validators, sequence blocks, and manage nodes.
- **Validator**: View keys, propose and approve blocks, store and retrieve data, and update validators.
- **Sequencer**: Propose blocks, sequence blocks, assign and remove shards.
- **Node**: Store and retrieve data, monitor network, manage nodes.
- **Observer**: View keys, retrieve data, monitor network.
- **User**: View keys, retrieve data.

### **Permissions**

- **Manage Keys**: Create, update, and delete cryptographic keys.
- **View Keys**: Access and view cryptographic keys.
- **Propose Blocks**: Submit new blocks for consensus.
- **Approve Blocks**: Approve proposed blocks.
- **Finalize Blocks**: Finalize blocks after reaching consensus.
- **Store Data**: Store data within the system.
- **Retrieve Data**: Retrieve stored data.
- **Assign Shard**: Assign validators to shards.
- **Remove Shard**: Remove validators from shards.
- **Monitor Network**: Observe network health and status.
- **Update Validator**: Update validator information.
- **Sequence Blocks**: Order or sequence blocks.
- **Manage Nodes**: Configure or manage node operations.