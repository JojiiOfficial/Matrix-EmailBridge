#!/bin/bash
echo "Be sure you have installed go >= 1.12 if not, press Ctrl+c"
sleep 4
go get
go build
./main
chmod 600 cfg.json data.db
mkdir ./temp
chmod 770 ./temp -R