#!/bin/bash

rm -rf client/build/web
mkdir -p client/build/web
godot --headless --path client --export-release "Web"
