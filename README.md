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

Put any CPTV files that you want to send to the fake camera in the directory fake-thermal-camera/fakecamera/cptv-files

## Browser Requests

### http://localhost:2040/sendCPTVFrames
*Send file / generated CPTV frames*

All query parameters are optional.  If you don't specify a file name it will try to use the file person.cptv

- cptv-file: {*string*} cptv-file to send (defaults to person.cptv)
- start: {*number*} first frame to send
- end: {*number*} frame to stop sending at
- generate: {*boolean*} whether or not to generate frames, if unspecified or false cptv-file will be used
- repeat: {*number*} number of times to repeat the sending of file or number of frames to generate (defaults to 1)
- minTemp: {*number*} min temp of frame (defaults to 3000)
- maxTemp: {*number*} max temp of frame (defaults to 4000)
- ffc: {*boolean*} if set to true, all generated / file frames will be ffc frames (defaults to false).
- enqueue: {*boolean*} whether to enqueue the sending of these frames (defaults to false).
- hotspots: {*JSON*}{*hotspot[]*} json array of spots to draw over the generated / file frames 
All the hotspot fields are mandatory, The top left of a frame is (0,0) while the bottom right is (width-1, height-1)
- hotspot:
    - shapeType: {*string*} "circle" or "rectangle"
    - x: {*number*} left position of the shape
    - y: {*number*} top position of the shape
    - width: {*number*} width of the shape 
    - height: {*number*} height of the shape
    - minTemp: {*number*} min temp of hotspot
    - maxTemp: {*number*} max temp of hotspot

#### Examples
1. `http://localhost:2040/sendCPTVFrames?repeat=10&hotspots=[{"shapeType":"circle","x":-5,"y":0,"width":20,"height":20,"minTemp":5000,"maxTemp":6000}]`
    - This draws a circle hotspot over every frame of the default cptv-file (person.cptv), this will be repeated 10 times (the CPTV file will be played back 10 times)
    - The hotspot will be the biggest circle that fits into the square, starting at top left (-5,0) with width 20 and height 20. The values of the hotspot will range between 5000 and 6000 

1. `http://localhost:2040/sendCPTVFrames?generate=True&hotspots=[{"shapeType":"rectangle","x":25,"y":30,"width":15,"height":50,"minTemp":4500,"maxTemp":4500}, {"shapeType":"circle","x":50,"y":50,"width":20,"height":40,"minTemp":5000,"maxTemp":6000}]`
    - This generates a single frame with pixel values ranging from 3000 - 4000 (default). A rectangle hotspot will be drawn on the frame with pixel values of 4500 starting at top left (25,30) with width 15 and height 50.
    - A oval hotspot will be drawn on the frame inside a rectangle defined by top left (50,50) width 20 and height 40.

### http://localhost:2040/clearCPTVQueue
*Clears all enqueued files / frames*
- stop: {*boolean*} stop sending of current frame

### http://localhost:2040/playback
*Controls the playback*

Query parameters:
 - stop: {*bool*} stops the current request (file / frames) playing
 - play: {*bool*} continues play if paused
 - clear: {*bool*} clears queue
 - pause: {*bool*} pauses the playback (only, play will resume it)
