# Stage 1: Build the Go application
FROM golang:1.23-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache make gcc musl-dev

# Set the working directory inside the container
WORKDIR /app

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
WORKDIR /app

# Copy the built executable from the builder stage
COPY --from=builder /app/build/fdb /app/fdb

# EXPOSE 0000

# Command to run the server
CMD ["./fdb", "serve"]
