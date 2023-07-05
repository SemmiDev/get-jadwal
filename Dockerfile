# Stage 1: Build
FROM golang:1.20-alpine AS build

# Set work directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod .
COPY go.sum .

# Download dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Copy vendor folder
COPY vendor/ vendor/

# Set GOFLAGS to use vendor mode
ENV GOFLAGS="-mod=vendor"

# Build the application
RUN CGO_ENABLED=0 go build -o main -ldflags="-s -w" .

# Stage 2: Run
FROM scratch

# Set work directory
WORKDIR /app

# Copy the built binary from the previous stage
COPY --from=build /app/main .

# Expose the port
EXPOSE 3030

# Run the application
CMD ["./main"]
