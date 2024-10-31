#!/bin/bash

# This script is executed by a container within a pod in the same place as ignition-server
# it will continue to run infinitelly and processing files which contains the commands to be executed

ALLOWED_BINARIES=("echo" "ls")
SLEEP_SECONDS=1

function validateCMDInput() {
    # Validates the input command to make sure it's safe to execute
    inputFile=$1

    if [[ -z "$inputFile" ]]; then
      echo "Error: missing file"
      return
    fi

    inputCMD=$(<"$inputFile")

    if [ -z "$inputCMD" ]; then
      echo "Error: missing input command"
      rm -f ${inputFile}
      return
    fi

    bin=$(echo $inputCMD | awk '{print $1}')

    allowed=false
    for allowedBin in "${ALLOWED_BINARIES[@]}"; do
        if [[ "$allowedBin" == "$bin" ]]; then
            allowed=true
            break
        fi
    done

    if [[ $allowed != true ]]; then
        echo "Error: command not allowed, deleting file..."
        rm -f $inputFile
    fi

    if [[ "$inputCMD" =~ [\&\&|\|\||\;] ]]; then
        echo "Input command contains invalid characters &&, ||, ;. Deleting file..."
        rm -f $inputFile
    fi
}

function basicValidations() {
    # Receive the path of the directory where the files will be parsed
    filesPath=$1

    echo "Validating path: $filesPath"
    if [ -z "$filesPath" ]; then
      echo "Error: missing argument"
      exit 1
    fi

    if [[ ! -d $filesPath ]]; then
        echo "Error: $1 is not a directory"
        exit 1
    fi

    if [[ ! -r $filesPath ]]; then
        echo "Error: $1 is not readable"
        exit 1
    fi
}

function parseFiles() {
    # Parse files from a location.
    # The script expects those files contains the
    # MCO commands for ignition payload rendering.
    # The script contains a validation layer for
    # security reasons.

    local dir=$1
    local files=$(ls $dir)
    for file in $files; do
        echo "Checking file: $file"
        validateCMDInput $dir/$file
        if [[ -f $dir/$file ]];then
            echo "Executing file: $file"
            execute $dir/$file
        fi
    done
}

function execute() {
  local file=$1
  local command=$(cat $file)
  echo "Executing command: $command"
  eval $command
  if [ $? -eq 0 ]; then
    echo "Command executed successfully"
  else
    echo "Error executing command, deleting file..."
  fi
  rm -f $file
  echo "-----"
}

function main() {
  basicValidations $1
  parseFiles $1
  while true; do
    parseFiles $1
    sleep $SLEEP_SECONDS
  done
}

main $1