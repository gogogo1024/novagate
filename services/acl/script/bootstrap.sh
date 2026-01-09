#!/bin/bash
CURDIR=$(cd $(dirname $0); pwd)
BinaryName=acl
echo "$CURDIR/bin/${BinaryName}"
exec $CURDIR/bin/${BinaryName}