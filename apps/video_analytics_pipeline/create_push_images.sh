#!/bin/bash

SCRIPT_DIR=$(dirname "$(realpath $0)")

# List of app names
apps=("annotate" "detect" "filter" "sink")

# Iterate over each app
for app in "${apps[@]}"; do
    # Go to the directory with the app name
    cd "$SCRIPT_DIR/$app" || { echo "Error: Directory $app not found"; exit 1; }

    # Execute a command with the app name (replace the command as needed)
    echo "Executing command for $app"
    docker build -t jyoo332/"$app"_server .
    docker push jyoo332/"$app"_server
    
    # Return to the original directory
    cd - > /dev/null
done