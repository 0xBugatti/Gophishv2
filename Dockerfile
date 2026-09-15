# ─────────────────────────────────────────────
# Stage 1 – Build JavaScript / CSS assets
# ─────────────────────────────────────────────
FROM node:26-alpine AS build-js

RUN npm install -g gulp gulp-cli

WORKDIR /build
COPY package.json yarn.lock ./
RUN npm install --only=dev

COPY . .
RUN gulp


# ─────────────────────────────────────────────
# Stage 2 – Compile Go binary
# ─────────────────────────────────────────────
FROM golang:1.24-bookworm AS build-golang

WORKDIR /go/src/github.com/gophish/gophish
COPY . .
# Bring in compiled frontend assets from stage 1
COPY --from=build-js /build/static/js/dist/  ./static/js/dist/
COPY --from=build-js /build/static/css/dist/ ./static/css/dist/

RUN go mod download && \
    CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o gophish .


# ─────────────────────────────────────────────
# Stage 3 – Lean runtime image
# ─────────────────────────────────────────────
FROM debian:bookworm-slim

# Runtime deps:
#   jq              – config patching in docker/run.sh
#   libcap2-bin     – setcap so gophish can bind port 80 without root
#   ca-certificates – TLS root CAs for outbound HTTPS/SMTP
#   python3/pip/venv – reports engine (DOCX / XLSX generation)
RUN apt-get update && \
    apt-get install --no-install-recommends -y \
        jq libcap2-bin ca-certificates \
        python3 python3-pip python3-venv && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Dedicated non-root user
RUN useradd -m -d /opt/gophish -s /bin/bash gophish

WORKDIR /opt/gophish

# Compiled binary + all runtime assets
COPY --from=build-golang /go/src/github.com/gophish/gophish/gophish         ./gophish
COPY --from=build-golang /go/src/github.com/gophish/gophish/config.json     ./config.json
COPY --from=build-golang /go/src/github.com/gophish/gophish/VERSION         ./VERSION
COPY --from=build-golang /go/src/github.com/gophish/gophish/templates/      ./templates/
COPY --from=build-golang /go/src/github.com/gophish/gophish/static/         ./static/
COPY --from=build-golang /go/src/github.com/gophish/gophish/db/             ./db/
COPY --from=build-golang /go/src/github.com/gophish/gophish/docker/         ./docker/
COPY --from=build-golang /go/src/github.com/gophish/gophish/reports/python  ./reports/python/

# Install Python report dependencies into an isolated venv
RUN python3 -m venv /opt/gophish/reports/venv && \
    /opt/gophish/reports/venv/bin/pip install --no-cache-dir \
        -r /opt/gophish/reports/python/requirements.txt

# Allow gophish to bind privileged ports (80, 443) as non-root
RUN setcap 'cap_net_bind_service=+ep' /opt/gophish/gophish

# Persistent SQLite database and generated report files
RUN mkdir -p /opt/gophish/data && \
    chown -R gophish:gophish /opt/gophish

USER gophish

# Bind admin to all interfaces and disable TLS (TLS handled by reverse proxy or
# re-enabled via ADMIN_USE_TLS env var at runtime)
RUN sed -i 's/127\.0\.0\.1/0.0.0.0/g' config.json && \
    sed -i 's/"use_tls": true/"use_tls": false/' config.json && \
    touch config.json.tmp

# Volume for persistent data (database + reports output)
VOLUME ["/opt/gophish/data"]

# 3333 = Admin UI
# 80   = Phishing server HTTP
# 443  = Phishing server HTTPS (optional)
EXPOSE 3333 80 443

CMD ["./docker/run.sh"]
