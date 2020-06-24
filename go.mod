module github.com/feverscreen/fake-thermal-camera

require (
	github.com/TheCacophonyProject/go-config v1.4.0
	github.com/TheCacophonyProject/go-cptv v0.0.0-20200616224711-fc633122087a
	github.com/TheCacophonyProject/lepton3 v0.0.0-20200617035048-c8720bf15f10
	github.com/TheCacophonyProject/thermal-recorder v1.22.1-0.20200617043743-4e0b47f00a8a
	github.com/alexflint/go-arg v1.1.0
	github.com/godbus/dbus v4.1.0+incompatible
	github.com/gorilla/mux v1.7.4
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0
	gopkg.in/yaml.v2 v2.2.8 // indirect
)

replace periph.io/x/periph => github.com/TheCacophonyProject/periph v2.1.1-0.20200615222341-6834cd5be8c1+incompatible

go 1.12
