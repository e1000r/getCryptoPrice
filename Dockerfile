# Use the official Go image to compile the application
FROM golang:1.23-alpine AS builder
RUN apk add --no-cache gcc musl-dev

# Set the working directory inside the container
WORKDIR /app

# Copy the configuration files and Go code into the container
COPY go.mod go.sum ./
RUN go mod tidy
COPY . .

# Compile the Go code into an executable
RUN go build -o app .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/app .
COPY --from=builder /app/.env .

# Port where the application will run (optional)
EXPOSE 8088

# Command to start the application
CMD ["./app"]
