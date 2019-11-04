#!/bin/bash

nslookup login.microsoftonline.com
nslookup management.azure.com

GODEBUG=netdns=cgo+2 /nalej/provisioner "$@"