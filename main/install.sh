#!/bin/bash
echo "Be sure you have installed go >= 1.12 if not, press Ctrl+c"
sleep 5
go get
go build
./main
