// fake-lepton - read a CPTV file and send it has raw frames to thermal-recorder
//  Copyright (C) 2020, The Cacophony Project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package fakecamera

import (
	"encoding/binary"
	"errors"
	"fmt"
	"gopkg.in/yaml.v1"
	"log"
	"math/rand"
	"net"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	goconfig "github.com/TheCacophonyProject/go-config"

	cptvframe "github.com/TheCacophonyProject/go-cptv/cptvframe"
	lepton3 "github.com/TheCacophonyProject/lepton3"
	"github.com/TheCacophonyProject/thermal-recorder/headers"
)

const (
	frameMinTemp = 3000
	frameMaxTemp = 4000
	sendSocket   = "/var/run/lepton-frames"

	sleepTime   = 30 * time.Second
	lockTimeout = 10 * time.Second
)

var (
	startTime     = time.Now()
	playCondition = sync.NewCond(&sync.Mutex{})
	stopSending   = false
	playing       = true
	cptvDir       string
	camera        cptvframe.CameraSpec
	queue         *Queue = newQueue()
)

func RunCamera(newCPTVDir, configDir string) error {
	cptvDir = newCPTVDir
	var err error
	camera, err = getCameraSpec(configDir)
	if err != nil {
		log.Printf("Error getting camera %v\n", err)
		return err
	}

	for {
		err = connectToSocket()
		if err != nil {
			fmt.Printf("Could not connect to socket %v will try again in %v\n", err, sleepTime)
			time.Sleep(sleepTime)
		} else {
			fmt.Print("Disconnected\n")
		}
	}
}

func getCameraSpec(configDir string) (cptvframe.CameraSpec, error) {
	configRW, err := goconfig.New(configDir)
	if err != nil {
		return nil, err
	}
	lepton := goconfig.DefaultLepton()
	if err := configRW.Unmarshal(goconfig.LeptonKey, &lepton); err != nil {
		return nil, err
	}
	camera = &lepton3.Lepton3{}
	return camera, nil
}

func connectToSocket() error {
	log.Printf("dialing frame output socket %s\n", sendSocket)
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{
		Net:  "unix",
		Name: sendSocket,
	})
	if err != nil {
		log.Printf("error %v\n", err)
		return errors.New("error: connecting to frame output socket failed")
	}
	defer conn.Close()
	conn.SetWriteBuffer(lepton3.FrameCols * lepton3.FrameCols * 2 * 20)

	camera_specs := map[string]interface{}{
		headers.YResolution: camera.ResY(),
		headers.XResolution: camera.ResX(),
		headers.FrameSize:   lepton3.BytesPerFrame,
		headers.Model:       "lepton3.5",
		headers.Brand:       "flir",
		headers.FPS:         camera.FPS(),
	}

	cameraYAML, _ := yaml.Marshal(camera_specs)
	if _, err := conn.Write(cameraYAML); err != nil {
		return err
	}

	conn.Write([]byte("\n"))
	log.Printf("Listening for send frames ...")
	return queueLoop(conn)
}

func Send(urlValues url.Values) {
	p := &params{urlValues}

	if !p.enqueue() {
		clearQueue(true)
		play()
	}
	queue.enqueue(p)
}

func queueLoop(conn *net.UnixConn) error {
	for {
		stopSending = false
		params := queue.dequeue()
		if params == nil {
			queue.wait()
			continue
		}

		maker, err := NewFrameMaker(params)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		sendFrames(conn, params, maker)
	}
}

func forceStop() {
	stopSending = true
	log.Println("Stopping")
}

func Playback(params url.Values) {
	stop, _ := strconv.ParseBool(params.Get("stop"))
	clear, _ := strconv.ParseBool(params.Get("clear"))

	if clear {
		clearQueue(stop)
		return
	}
	if stop {
		forceStop()
		return
	}

	pauseB, _ := strconv.ParseBool(params.Get("pause"))
	if pauseB {
		pause()
		return
	}
	playB, _ := strconv.ParseBool(params.Get("play"))
	if playB {
		play()
		return
	}
}

func play() {
	log.Println("Playing")
	playCondition.L.Lock()
	playing = true
	playCondition.L.Unlock()
	playCondition.Signal()

}

func pause() {
	log.Println("Pausing")
	playCondition.L.Lock()
	playing = false
	playCondition.L.Unlock()
}

func clearQueue(stop bool) {
	queue.clear()
	if stop {
		forceStop()
	}
	log.Println("QueueCleared")

}

func generatePixel(minTemp, maxTemp int) uint16 {
	var pix int
	if maxTemp <= minTemp {
		pix = minTemp
	} else {
		pix = minTemp + rand.Intn(maxTemp-minTemp)
	}
	return uint16(pix)
}

func waitForPlay() {
	playCondition.L.Lock()
	if !playing {
		playCondition.Wait()
	}
	playCondition.L.Unlock()
}

func sendFrames(conn *net.UnixConn, params *params, f *frameMaker) error {
	defer f.Close()
	// Telemetry size of 640 -64(size of telemetry words)
	var reaminingBytes [576]byte
	frameSleep := time.Duration(1000/f.fps) * time.Millisecond
	for {
		if !playing {
			waitForPlay()
		}
		if stopSending {
			return nil
		}

		frame, err := f.NextFrame()
		if err != nil {
			break
		}

		buf := rawTelemetryBytes(frame.Status)
		_ = binary.Write(buf, binary.BigEndian, reaminingBytes)
		for _, row := range frame.Pix {
			for x, _ := range row {
				_ = binary.Write(buf, binary.BigEndian, row[x])
			}
		}
		// replicate cptv frame rate
		if _, err := conn.Write(buf.Bytes()); err != nil {
			// reconnect to socket
			return err
		}
		time.Sleep(frameSleep)
	}
	return nil
}
