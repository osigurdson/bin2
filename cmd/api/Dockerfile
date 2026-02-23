FROM docker.io/library/debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY api .
RUN chmod +x ./api
CMD ["/app/api"]
