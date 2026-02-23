FROM docker.io/library/debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY init .
RUN chmod +x ./init
CMD ["/app/init", "migrate"]
