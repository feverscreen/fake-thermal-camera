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

## Browser Requests

### http://localhost:2040/sendCPTVFrames
*Send cptv /generated CPTV frames*

All query params are optional.  If you don't specify a filename it will try to use the file person.cptv

- cptv-file: {string} cptv-file to send (defaults to person.cptv)
- start: {number} first frame to send
- end: {number} frame to stop sending at
- generate: {boolean} weather or not to generate frames, if unspecified or false cptv-file will be used
- repeat: {number} number of times to repeat the sending of file or number of frames to generate
- minTemp: {number} min temp of frame
- maxTemp: {number} max temp of frame
- ffc: {boolean} if set to true, all generated / cptv frames will be ffc frames
- enqueue: {boolean} weather to enqueue the sending of these frames
- hotspots: {hotspot[]} array of spots to draw over the generated / cptv frames 
- hotspot:
    - shapeType: {string} "circle" or "rectangle"
    - x: {number} left position of the shape
    - y: {number} top position of the shape
    - width: {number} width of the shape 
    - height: {number} height of the shape
    - minTemp: {number} min temp of hotspot
    - maxTemp: {number} max temp of hotspot


### http://localhost:2040/clearCPTVQueue
*Clears all enqeued files / frames*
- stop: {boolean} stop sending of current frame
