# Use an official Go runtime as a parent image
FROM golang:latest

# Set the working directory in the container
WORKDIR /app

# Copy the Go server source code into the container
COPY . .

# Build the Go server binary
RUN go build -o main .

# Expose the port the server will run on
EXPOSE 8080

# Define the command to run the Go server
CMD ["./main"]
