# Use the official Golang image as the base image
FROM golang:1.24-alpine AS builder

# Set the working directory
WORKDIR /app

# Install necessary build tools
RUN apk add --no-cache git gcc musl-dev

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Run tests before building the application
RUN go test ./...

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o airpollutionbot .

# Use a smaller base image for the final stage
FROM alpine:latest

# Set the working directory
WORKDIR /root/
# Copy the binary from the builder stage
COPY --from=builder /app/airpollutionbot .

# Expose port 8080
EXPOSE 8080

# Run the application
CMD ["./airpollutionbot"]
