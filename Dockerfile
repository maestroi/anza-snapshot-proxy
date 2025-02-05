# Build stage
FROM golang:1.23-alpine AS builder

# Install necessary dependencies
RUN apk update && apk add --no-cache git

# Set the working directory
WORKDIR /app

# Copy the Go modules and download dependencies
COPY go.mod ./
RUN go mod tidy

# Copy the source code
COPY . .

# Build the Go application
RUN go build -o anza-proxy .

# Run stage
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the built application from the builder stage
COPY --from=builder /app/anza-proxy  .
COPY --from=builder /app/config.json .

# Expose the necessary port
EXPOSE 14705

# Command to run the application
CMD ["./anza-proxy"]
