#!/bin/bash

set -e

echo --starting dbus --
dbus-daemon --config-file=/usr/share/dbus-1/system.conf --print-address

echo --- fever-screen ----
cd /code/feverscreen
echo Building fever-fever....
make build 

echo --- starting supervisord ---
/usr/bin/supervisord &
disown

echo '*************************************************'
echo 'Fever screen will be available on http://localhost:2041'
echo 'The test server will be available on http://localhost:2040 (after build finishes)'
echo 'See README.md for how to use the server'
echo '*************************************************'

echo --- test-server ----
cd /server/cmd/testing-server/
echo Building and running test-server....
go get github.com/markbates/refresh
refresh init -c refresh.yml
refresh run