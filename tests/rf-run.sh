#!/bin/sh

robot --consolecolors on -r none -l ./tests/out/$(basename $1)-log --output ./tests/out/$(basename $1)-out $1