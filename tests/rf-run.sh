#!/bin/sh

robot -r none -l ./tests/out/$(basename $1)-log --output ./tests/out/$(basename $1)-out $1