# Start from the official Golang image
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o airpollutionbot .

# Use a minimal alpine image for the final stage
FROM alpine:3.19

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create and set permissions for app directory
WORKDIR /app
RUN chown -R appuser:appgroup /app

# Copy the binary from the builder stage
COPY --from=builder /app/airpollutionbot .

# Copy any additional configuration files if needed
# COPY --from=builder /app/config.yaml .

# Set ownership of all files to the non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose any necessary ports (if your bot needs to expose a port)
# EXPOSE 8080

# Command to run the executable
CMD ["./airpollutionbot"]