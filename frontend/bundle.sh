#!/bin/bash

# a very temporary script for copying css and images into the static directory

STATIC_DIR="../static"

if [ $# -eq 0 ]; then
	cp -r -v ./src/* $STATIC_DIR/.

	exit
fi

if [ $1 == "help" ]; then
	echo "A very temporary script for copying css and images into the 'static' directory"
	echo "Calling this command with no arguments will simply copy everything from ./src into static"
	exit
fi
