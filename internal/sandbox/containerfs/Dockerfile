# The sandbox agent: a tiny Go program that speaks a newline-delimited JSON protocol over
# stdin/stdout, implementing the file/shell operations the host-side isolated sandbox needs.
# See sandbox_isolated_container.go (host) and main.go (this program) for the protocol.
#
# Built in its own stage so the toolchain used to compile it doesn't need to match (or pollute)
# the runtime image's.
FROM golang:1.27.1-bookworm AS agent-builder
WORKDIR /opt/sandbox-agent
COPY _go.mod ./go.mod
COPY *.go ./
ENV CGO_ENABLED=0
RUN go build -o /usr/local/bin/sandbox-agent .

FROM debian:bookworm-slim
RUN apt update && apt install -y ripgrep python3 git build-essential curl wget && \
    curl -fsSL https://deb.nodesource.com/setup_24.x | bash - && \
    apt install -y nodejs && \
    wget https://go.dev/dl/go1.27.1.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.27.1.linux-amd64.tar.gz && \
    rm go1.27.1.linux-amd64.tar.gz && \
    apt clean && \
    rm -rf /var/lib/apt/lists/*
ENV PATH=$PATH:/usr/local/go/bin:

COPY --from=agent-builder /usr/local/bin/sandbox-agent /usr/local/bin/sandbox-agent

ENTRYPOINT ["/usr/local/bin/sandbox-agent"]
