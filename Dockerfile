FROM scratch

COPY portmap /portmap

USER 65532:65532

ENTRYPOINT ["/portmap"]
