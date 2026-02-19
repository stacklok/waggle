#!/bin/sh

path=$1
header=$2

printf '%s\n' "$header"

if find "$path" -maxdepth 1 -mindepth 1 -printf '%y|%M|%s|%T@|%f\n' >/dev/null 2>&1; then
  find "$path" -maxdepth 1 -mindepth 1 -printf '%y|%M|%s|%T@|%f\n' 2>/dev/null
  exit 0
fi

if stat -c '%A|%s|%Y' -- "$path" >/dev/null 2>&1; then
  find "$path" -maxdepth 1 -mindepth 1 -exec sh -c '
    for entry do
      if [ -L "$entry" ]; then type=l
      elif [ -d "$entry" ]; then type=d
      else type=f
      fi
      perms=$(stat -c "%A" -- "$entry" 2>/dev/null) || perms="?"
      size=$(stat -c "%s" -- "$entry" 2>/dev/null) || size=0
      mtime=$(stat -c "%Y" -- "$entry" 2>/dev/null) || mtime=0
      name=$(basename -- "$entry")
      printf "%s|%s|%s|%s|%s\n" "$type" "$perms" "$size" "$mtime" "$name"
    done
  ' sh {} +
  exit 0
fi

if stat -f '%Sp|%z|%m' -- "$path" >/dev/null 2>&1; then
  find "$path" -maxdepth 1 -mindepth 1 -exec sh -c '
    for entry do
      if [ -L "$entry" ]; then type=l
      elif [ -d "$entry" ]; then type=d
      else type=f
      fi
      perms=$(stat -f "%Sp" -- "$entry" 2>/dev/null) || perms="?"
      size=$(stat -f "%z" -- "$entry" 2>/dev/null) || size=0
      mtime=$(stat -f "%m" -- "$entry" 2>/dev/null) || mtime=0
      name=$(basename -- "$entry")
      printf "%s|%s|%s|%s|%s\n" "$type" "$perms" "$size" "$mtime" "$name"
    done
  ' sh {} +
  exit 0
fi

exit 1
