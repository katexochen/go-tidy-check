#!/bin/bash
major=${1%%.*}
git tag -d $1
git tag -d $major
git push --delete origin $1
git push --delete origin $major
