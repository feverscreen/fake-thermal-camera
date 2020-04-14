#!/bin/bash

echo --starting dbus --
dbus-daemon --config-file=/usr/share/dbus-1/system.conf --print-address

echo --- fever-screen ----
cd /code/feverscreen
echo Building fever-fever....
make build ./...
cp    "../feverscreen/webserver/_release/managementd-avahi.service" "/etc/avahi/services/managementd.service"

echo --- fake-lepton ----
cd /server
echo Building fake-lepton....
cd cmd/fake-lepton/
go build
cp org.cacophony.FakeLepton.conf /etc/dbus-1/system.d/org.cacophony.FakeLepton.conf


echo --- starting supervisord ---
/usr/bin/supervisord &
disown

echo --- test-server ----
cd /server/cmd/testing-server/
echo Building and running test-server....
# go get github.com/markbates/refresh
refresh init -c refresh.yml
refresh run
