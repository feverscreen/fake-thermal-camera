# fake thermal camera

`fake-thermal-camera` is a server that runs some of the thermal camera code (currently on linux not raspberian).   Is is designed for using with the cypress integration tests.

Project | fake-thermal-camera
---|---
Platform | Linux
Requires | Git repository [`feverscreen`](https://github.com/feverscreen/feverscreen)
Licence | GNU General Public License v3.0

## Development Instructions

Download fake-thermal-camera and feverscreen into the same folder.

In the fake-thermal-camera folder start the test server with
```
> ./run
```

Open up http://localhost:2041/ to see the feverscreen display.

Put any cptv files that you want to send to the fake camera in the directory fake-thermal-camera/cmd/fake-lepton/cptv-files

Then in your browser call http://localhost:2040/sendCPTVFrames/?cptv-file={filename}
If you don't specify a filename the default one will be played.
