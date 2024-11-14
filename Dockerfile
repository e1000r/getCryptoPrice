# Use the official Go image to compile the application
FROM golang:1.23 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the configuration files and Go code into the container
COPY go.mod go.sum ./
RUN go mod tidy

COPY . .

# Compile the Go code into an executable
RUN go build -o app .

# Copy the .env file
COPY .env .

# Port where the application will run (optional)
EXPOSE 8080

# Command to start the application
CMD ["./app"]
