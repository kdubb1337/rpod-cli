FROM gcr.io/distroless/static-debian12
COPY rpod /usr/local/bin/rpod
ENTRYPOINT ["/usr/local/bin/rpod"]
