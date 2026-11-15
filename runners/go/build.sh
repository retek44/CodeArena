#!/bin/sh
# Compiles main.go into the static binary main against the standard library that the
# image compiled at build time (/opt/codearena-go/importcfg). Runs inside the sandbox.
set -eu
# The tools must use the same experiments as the go command that built the library.
. /opt/codearena-go/env
export GOEXPERIMENT
cfg=/opt/codearena-go/importcfg
tools=/opt/codearena-go/tools
"${tools}/compile" -p main -complete -importcfg "${cfg}" -trimpath "${PWD}" -o main.a main.go
"${tools}/link" -importcfg "${cfg}" -buildmode=exe -s -w -o main main.a
rm -f main.a
