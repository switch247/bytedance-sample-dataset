FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
	ca-certificates curl wget git build-essential tar python3 python3-pip \
	&& rm -rf /var/lib/apt/lists/*

ENV GOVER=1.22.5
RUN curl -fsSL https://go.dev/dl/go${GOVER}.linux-amd64.tar.gz -o /tmp/go.tgz \
	&& tar -C /usr/local -xzf /tmp/go.tgz \
	&& rm /tmp/go.tgz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app
COPY . /app

# Install Python dependencies (always run to populate cache)
RUN pip3 install --no-cache-dir --break-system-packages -r /app/requirements.txt

# Prepare Go modules (always run to populate module cache)
RUN go env -w GOPROXY=https://proxy.golang.org && cd /app && go mod tidy

# Allow direct command execution via `docker run` / `docker compose run`
ENTRYPOINT []
CMD ["sleep", "infinity"]


