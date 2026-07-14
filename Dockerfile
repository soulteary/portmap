FROM scratch

COPY portmap /portmap

ENTRYPOINT ["/portmap"]
