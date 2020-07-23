module github.com/feverscreen/fake-thermal-camera

replace periph.io/x/periph => github.com/TheCacophonyProject/periph v1.0.1-0.20200331204442-4717ddfb6980

go 1.12

require (
	github.com/TheCacophonyProject/go-config v1.4.0
	github.com/TheCacophonyProject/go-cptv v0.0.0-20200225002107-8095b1b6b929
	github.com/TheCacophonyProject/lepton3 v0.0.0-20200430221413-9df342ce97f9
	github.com/TheCacophonyProject/thermal-recorder v1.22.1-0.20200225033227-2090330c5c11
	github.com/alexflint/go-arg v1.3.0
	github.com/godbus/dbus v4.1.0+incompatible
	github.com/gorilla/mux v1.7.4
	github.com/stretchr/testify v1.4.0 // indirect
	golang.org/x/net v0.0.0-20200513185701-a91f0712d120 // indirect
	golang.org/x/sys v0.0.0-20200513112337-417ce2331b5c // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0
	gopkg.in/yaml.v2 v2.3.0 // indirect
)
