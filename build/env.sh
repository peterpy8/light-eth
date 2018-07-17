#!/bin/sh

set -e

if [ ! -f "build/env.sh" ]; then
    echo "$0 must be run from the root of the repository."
    exit 2
fi

# Create fake Go workspace if it doesn't exist yet.
workspace="$PWD/build/_workspace"
root="$PWD"
siotdir="$workspace/src/github.com/siotchain"
if [ ! -L "$siotdir/siot" ]; then
    mkdir -p "$siotdir"
    cd "$siotdir"
    ln -s ../../../../../. siot
    cd "$root"
fi

# Set up the environment to use the workspace.
GOPATH="$workspace"
export GOPATH

# Run the command inside the workspace.
cd "$siotdir/siot"
PWD="$siotdir/siot"

# Launch the arguments with the configured environment.
exec "$@"
