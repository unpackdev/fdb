# eBPF Makefile for handling eBPF-related tasks

# Variables
CLANG := clang
BPFTOOL := bpftool
SRC_DIR := c/src
OBJ_DIR := c/obj
SRC := $(SRC_DIR)/ebpf_program.c
OBJ := $(OBJ_DIR)/ebpf_program.o
INTERFACE := eth0 # Replace with the network interface you want to attach to

# eBPF targets
.PHONY: ebpf-all ebpf-build ebpf-load ebpf-unload ebpf-status ebpf-clean

# Default eBPF target
ebpf-all: ebpf-build

# Create object directory if it doesn't exist
$(OBJ_DIR):
	mkdir -p $(OBJ_DIR)

# Compile the eBPF program
$(OBJ): $(SRC) | $(OBJ_DIR)
	$(CLANG) -O2 -g -target bpf -c $(SRC) -o $(OBJ)

# Build the eBPF program
ebpf-build: $(OBJ)
	@echo "eBPF program compiled successfully."

# Load the eBPF program onto the interface
ebpf-load: $(OBJ)
	sudo ip link set dev $(INTERFACE) xdp obj $(OBJ) sec xdp
	@echo "eBPF program loaded onto interface $(INTERFACE)."

# Unload the eBPF program from the interface
ebpf-unload:
	sudo ip link set dev $(INTERFACE) xdp off
	@echo "eBPF program unloaded from interface $(INTERFACE)."

# Check if the eBPF program is loaded
ebpf-status:
	$(BPFTOOL) prog

# Clean up the compiled objects
ebpf-clean:
	rm -rf $(OBJ_DIR)
	@echo "Cleaned up eBPF compiled files."