# Stage 1: Build the Go application
FROM golang:1.23-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache make git gcc musl-dev

# Set the working directory inside the container
WORKDIR /fdb

# Copy the Go modules manifest and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the project files
COPY . .

# Build the application using make
RUN make build

# Stage 2: Run the Go application
FROM alpine:latest

# Set the working directory inside the container
WORKDIR /fdb

# Copy the built executable from the builder stage
COPY --from=builder /fdb/build/fdb /fdb/fdb

# Copy the YAML configuration file
COPY config.yaml /fdb/config.yaml

# Copy the certificate files
COPY data/certs/ /fdb/data/certs/

# Expose the necessary ports from the config file
EXPOSE 4434 4433 5011 5022 4060

# Command to run the server with the configuration file
CMD ["./fdb", "serve", "--config", "./config.yaml"]
