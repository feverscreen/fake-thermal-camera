package main

import (
    "errors"
    "fmt"
    "net"
    "net/url"

    "github.com/godbus/dbus"
    "github.com/godbus/dbus/introspect"
)

const (
    dbusName = "org.cacophony.FakeLepton"
    dbusPath = "/org/cacophony/FakeLepton"
)

type service struct {
    conn *net.UnixConn
}

func startService(unixConn *net.UnixConn) (*service, error) {
    conn, err := dbus.SystemBus()
    if err != nil {
        return nil, err
    }
    reply, err := conn.RequestName(dbusName, dbus.NameFlagDoNotQueue)
    if err != nil {
        return nil, err
    }
    if reply != dbus.RequestNameReplyPrimaryOwner {
        return nil, errors.New("name already taken")
    }

    s := &service{conn: unixConn}
    conn.Export(s, dbusPath, dbusName)
    conn.Export(genIntrospectable(s), dbusPath, "org.freedesktop.DBus.Introspectable")
    return s, nil
}

func genIntrospectable(v interface{}) introspect.Introspectable {
    node := &introspect.Node{
        Interfaces: []introspect.Interface{{
            Name:    dbusName,
            Methods: introspect.Methods(v),
        }},
    }
    return introspect.NewIntrospectable(node)
}

// SendCPTV will send the raw frames of a cptv, to thermal-recorder
func (s *service) SendCPTV(params url.Values) *dbus.Error {
    fmt.Printf("Recieved cptv %v\n", params["filename"])
    send(params)
    return nil
}
