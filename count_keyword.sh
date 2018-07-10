#!/bin/sh
dirlist=$(find -mindepth 1 -maxdepth 1 -type d)

for dir in $dirlist
do
  cd $dir
  echo $dir
  grep -i $1 -r | wc -l  
  cd ..
done
