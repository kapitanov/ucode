FROM debian:bookworm-slim
RUN apt update && apt install -y ripgrep python3 git build-essential curl wget && \
    curl -fsSL https://deb.nodesource.com/setup_24.x | bash - && \
    apt install -y nodejs && \
    wget https://go.dev/dl/go1.26.5.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.26.5.linux-amd64.tar.gz && \
    rm go1.26.5.linux-amd64.tar.gz && \
    apt clean && \
    rm -rf /var/lib/apt/lists/*
ENV PATH=$PATH:/usr/local/go/bin:
