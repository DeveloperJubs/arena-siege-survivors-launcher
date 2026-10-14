# Cross-compiles the launcher for Windows. Fyne needs cgo, so this adds the MinGW-w64 C compiler
# to the Go image. Used by "make build" when MinGW-w64 is not installed locally.
FROM golang:1.26-bookworm
RUN apt-get update \
 && apt-get install -y --no-install-recommends gcc-mingw-w64-x86-64 \
 && rm -rf /var/lib/apt/lists/*
