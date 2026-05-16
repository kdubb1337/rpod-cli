FROM gcr.io/distroless/static-debian12
COPY runpod /usr/local/bin/runpod
ENTRYPOINT ["/usr/local/bin/runpod"]
