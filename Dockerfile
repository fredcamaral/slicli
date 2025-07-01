# Multi-stage build for minimal final image
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application and plugins
RUN make build build-plugins

# Final stage - minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    chromium \
    chromium-chromedriver \
    ttf-dejavu \
    fontconfig \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN adduser -D -s /bin/sh slicli

# Set up directories
RUN mkdir -p /usr/local/lib/slicli/plugins \
    && mkdir -p /usr/local/share/slicli/themes \
    && mkdir -p /home/slicli/.config/slicli \
    && chown -R slicli:slicli /home/slicli

# Copy built artifacts from builder stage
COPY --from=builder /app/bin/slicli /usr/local/bin/
COPY --from=builder /app/plugins /usr/local/lib/slicli/plugins/
COPY --from=builder /app/themes /usr/local/share/slicli/themes/
COPY --from=builder /app/configs/default.toml /usr/local/share/slicli/

# Set up Chrome for headless mode
ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROME_PATH=/usr/bin/chromium-browser

# Switch to non-root user
USER slicli
WORKDIR /home/slicli

# Set default config path
ENV SLICLI_CONFIG_PATH=/usr/local/share/slicli/default.toml
ENV SLICLI_THEMES_PATH=/usr/local/share/slicli/themes
ENV SLICLI_PLUGINS_PATH=/usr/local/lib/slicli/plugins

# Expose default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD slicli --version || exit 1

# Default command
ENTRYPOINT ["slicli"]
CMD ["--help"]