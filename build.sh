#!/bin/bash

# build Linux
echo building Linux...
env GOOS=linux GOARCH=amd64 go build -o ./build/linux_amd64/mailrelay-linux-amd64

# build Windows
echo building Windows...
env GOOS=windows GOARCH=amd64 go build -o ./build/windows_amd64/mailrelay-windows-amd64.exe

# build OSX
echo building OSX...
env GOOS=darwin GOARCH=amd64 go build -o ./build/osx_amd64/mailrelay-osx-amd64

# build OpenBSD
echo building OpenBSD...
env GOOS=openbsd GOARCH=amd64 go build -o ./build/openbsd_amd64/mailrelay-openbsd-amd64

# build Linux ARM64 (Raspberry PI)
echo "building Linux ARM64 (Raspberry PI)..."
env GOOS=linux GOARCH=arm go build -o ./build/linux_amd64/mailrelay-linux-arm64