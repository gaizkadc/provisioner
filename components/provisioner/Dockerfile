#
# Copyright (C) 2018 Nalej Group - All Rights Reserved
#

FROM alpine:3.9.4

# Now just add the binary\
RUN mkdir -p /nalej/resources
# Copy the resources
COPY dockerenv/cert-manager*.yaml /nalej/resources/

# Now just add the binary
COPY provisioner /nalej/


ENTRYPOINT ["./nalej/provisioner"]
