#!/bin/bash

nslookup login.microsoftonline.com
nslookup management.azure.com

GODEBUG=netdns=2 /nalej/provisioner "$@"