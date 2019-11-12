
FROM alpine:3.9.4

COPY dockerenv/cert-manager*.yaml /nalej/resources/
COPY provisioner /nalej/

ENTRYPOINT ["./nalej/provisioner"]
