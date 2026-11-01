#!/bin/bash

rm -rf client/build/web
mkdir -p client/build/web
export XDG_DATA_HOME=$PWD/.cache/godot-xdg/data
export XDG_CONFIG_HOME=$PWD/.cache/godot-xdg/config
/home/tfkfan/godot --headless --path client --export-release "Web"
