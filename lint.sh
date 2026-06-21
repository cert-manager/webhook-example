# This script is intended to be sourced like "source lint.sh" or ". lint.sh".
curl --silent https://raw.githubusercontent.com/JenswBE/setup/main/programming_configs/golang/.golangci.yml -o .golangci.yml
golangci-lint run --disable funcorder
rm .golangci.yml
