ARG BASE_IMAGE=golang:1.26
FROM ${BASE_IMAGE} AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY . .

# GOARCH is left unset so the binary matches the build platform
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o agent-factory-controller ./cmd/agent-factory-controller

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/agent-factory-controller .
USER 65532:65532

ENTRYPOINT ["/agent-factory-controller"]
