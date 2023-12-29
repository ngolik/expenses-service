# Use the official Golang image as the base image
FROM golang:latest as builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go modules manifests
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

COPY .env /app/.env

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o app

# Use a minimal base image for the final image
FROM alpine:latest

# Set the working directory inside the container
WORKDIR /app

# Copy only the necessary files from the builder image
COPY --from=builder /app/app .

# Expose the port the application runs on
EXPOSE 8082

# Command to run the application
CMD ["./app"]
