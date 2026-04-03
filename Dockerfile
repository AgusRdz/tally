FROM golang:1.24-alpine

RUN apk add --no-cache git curl

ARG GIT_CLIFF_VERSION=2.12.0
RUN ARCH=$(uname -m) && \
    curl -fsSL "https://github.com/orhun/git-cliff/releases/download/v${GIT_CLIFF_VERSION}/git-cliff-${GIT_CLIFF_VERSION}-${ARCH}-unknown-linux-musl.tar.gz" \
    | tar -xz -C /tmp && \
    find /tmp -name "git-cliff" -type f | head -1 | xargs -I{} mv {} /usr/local/bin/git-cliff && \
    chmod +x /usr/local/bin/git-cliff

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

COPY . .

CMD ["go", "build", "-o", "bin/budget", "."]
