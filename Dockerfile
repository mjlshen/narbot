# Use the offical golang image to create a binary.
# This is based on Debian and sets the GOPATH to /go.
# https://hub.docker.com/_/golang
FROM golang:1.15-buster as builder

# Create and change to the app directory.
WORKDIR /app

# Retrieve application dependencies.
# This allows the container build to reuse cached dependencies.
# Expecting to copy go.mod and if present go.sum.
COPY go.* ./
RUN go mod download

# Copy local code to the container image.
COPY . ./

# Build the binary.
WORKDIR /app/cmd/narbot
RUN CGO_ENABLED=0 go build -mod=readonly -v -o narbot

FROM gcr.io/distroless/static

# Copy the binary to the production image from the builder stage.
COPY --from=builder /app/cmd/narbot/narbot /app/narbot
COPY --from=builder /app/config.json /config.json

# Run the web service on container startup.
CMD ["/app/narbot"]
