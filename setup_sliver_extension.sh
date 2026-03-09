#!/bin/bash

# ligolo-ng-bof Sliver Extension Setup Script
# This script creates the proper directory structure for Sliver extension

EXTENSIONS_DIR="$HOME/.sliver-client/extensions"
PROJECT_DIR="$(pwd)"

echo "Setting up ligolo-ng-bof extension in: $EXTENSIONS_DIR"
echo "Source directory: $PROJECT_DIR"
echo ""

# Build extension first
echo "Building extension..."
if command -v make &> /dev/null; then
    make clean all windowsdll_64 windowsdll_32
else
    echo "  ⚠ Warning: make not found. Assuming binaries are already built."
fi
echo ""

# Ensure extensions directory exists
mkdir -p "$EXTENSIONS_DIR"

ext_file="extension.json"

if [ ! -f "$ext_file" ]; then
    echo "ERROR: extension.json not found!"
    exit 1
fi

# Extract the name from the extension.json file
name=$(jq -r '.name' "$ext_file")

if [ -z "$name" ] || [ "$name" = "null" ]; then
    echo "ERROR: Could not extract name from $ext_file"
    exit 1
fi

echo "Processing extension: $name"

# Create the extension directory
ext_dir="$EXTENSIONS_DIR/$name"
mkdir -p "$ext_dir"

# Copy the extension.json file
cp "$ext_file" "$ext_dir/extension.json"
echo "  ✓ Copied extension.json"

# Get the extension paths from the extension.json
file_paths=$(jq -r '.files[].path' "$ext_file")

# Flag to track if all files were copied
all_files_found=true

# Copy binary files
for rel_path in $file_paths; do
    filename=$(basename "$rel_path")
    source_file="$PROJECT_DIR/$filename"
    dest_file="$ext_dir/$filename"
    
    if [ -f "$source_file" ]; then
        cp "$source_file" "$dest_file"
        echo "  ✓ Copied: $filename"
    else
        echo "  ✗ WARNING: File not found: $source_file"
        all_files_found=false
    fi
done

echo ""
if [ "$all_files_found" = true ]; then
    echo "✓ Extension setup complete: $name"
    echo "Extension ready at: $ext_dir"
else
    echo "⚠ Extension setup incomplete: $name (some files missing)"
    exit 1
fi

echo ""
echo "Use 'extensions load $ext_dir' in Sliver to load this extension."
