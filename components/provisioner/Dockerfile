#
# Copyright (C) 2018 Nalej Group - All Rights Reserved
#

FROM alpine:3.9.4

# Now just add the binary\
RUN mkdir /nalej

# Now just add the binary
COPY provisioner /nalej/

ENTRYPOINT ["./nalej/provisioner"]