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
    "bytes"
    "encoding/binary"
    "encoding/json"
    "errors"
    "fmt"
    "gopkg.in/yaml.v1"
    "io"
    "log"
    "math"
    "math/rand"
    "net"
    "net/url"
    "os"
    "path"
    "strconv"
    "sync"
    "time"

    goconfig "github.com/TheCacophonyProject/go-config"

    "github.com/TheCacophonyProject/go-cptv"
    cptvframe "github.com/TheCacophonyProject/go-cptv/cptvframe"
    lepton3 "github.com/TheCacophonyProject/lepton3"
    "github.com/TheCacophonyProject/thermal-recorder/headers"
)

const (
    frameMinTemp = 3000
    frameMaxTemp = 4000
    sendSocket   = "/var/run/lepton-frames"
    queueSleep   = 1 * time.Second

    sleepTime   = 30 * time.Second
    lockTimeout = 10 * time.Second
)

var (
    startTime                 = time.Now()
    lockChannel chan struct{} = make(chan struct{}, 1)
    wg          sync.WaitGroup
    cptvDir     = "/cptv-files"

    stopSending = false

    queueLock sync.Mutex
    sendQueue = make([]url.Values, 0, 3)
    camera    cptvframe.CameraSpec
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
        headers.Model:       lepton3.Model,
        headers.Brand:       lepton3.Brand,
        headers.FPS:         camera.FPS(),
    }

    cameraYAML, err := yaml.Marshal(camera_specs)
    if _, err := conn.Write(cameraYAML); err != nil {
        return err
    }

    conn.Write([]byte("\n"))
    log.Printf("Listening for send frames ...")
    return queueLoop(conn)
}

func enqueue(params url.Values) {
    queueLock.Lock()
    defer queueLock.Unlock()
    sendQueue = append(sendQueue, params)
}

func dequeue() url.Values {
    queueLock.Lock()
    defer queueLock.Unlock()
    if len(sendQueue) == 0 {
        return nil
    }
    top := sendQueue[0]
    sendQueue = sendQueue[1:]
    return top
}

func Send(params url.Values) {
    enq, err := strconv.ParseBool(params.Get("enqueue"))
    if err != nil {
        enq = false
    }
    if !enq {
        ClearQueue(true)
    }
    enqueue(params)
}

func queueLoop(conn *net.UnixConn) error {
    for {
        stopSending = false
        params := dequeue()
        if params == nil {
            time.Sleep(queueSleep)
            continue
        }
        repeat, _ := strconv.Atoi(params.Get("repeat"))
        if repeat == 0 {
            repeat = 1
        }

        generate, err := strconv.ParseBool(params.Get("generate"))
        if err != nil {
            generate = false
        }
        if generate {
            sendFrames(conn, params, repeat)
        } else {
            for i := 0; i < repeat; i++ {
                err := sendCPTV(conn, params)
                if err != nil {
                    return err
                }
            }
        }
    }
}

func forceStop() {
    stopSending = true
}

func ClearQueue(stop bool) {
    queueLock.Lock()
    defer queueLock.Unlock()
    sendQueue = make([]url.Values, 0, 3)
    if stop {
        forceStop()
    }
}

type shape interface {
    // gets the 2 x intersections points of this line (y) with the shape
    intersections(h *spot, y int) (int, int)
}

type rectangle struct {
}

func (r rectangle) intersections(h *spot, y int) (int, int) {
    return h.X, h.X + h.Width
}

type circle struct {
    h, k, a, b float64
}

func NewCircle(spot *spot) circle {
    c := circle{}
    c.h = float64(spot.X) + float64(spot.Width)/2.0
    c.k = float64(spot.Y) + float64(spot.Height)/2.0
    // these are already squared for convenience
    c.a = math.Pow(float64(spot.Width)/2.0, 2)
    c.b = math.Pow(float64(spot.Height)/2.0, 2)
    return c
}

// intersections of a line with a circle solves the equations
// (x-h)^2/a + (y-k)^2/b = 1
func (r circle) intersections(h *spot, y int) (int, int) {
    res := math.Sqrt(r.a * (1 - math.Pow(float64(y)-r.k, 2)/r.b))
    return int(math.Ceil(-res + r.h)), int(math.Floor(res + r.h))
}

type hotspot struct {
    spot  *spot
    shape shape
}

func (h hotspot) addSpot(pix [][]uint16) {
    height := len(pix)
    width := len(pix[0])
    yStart := int(math.Max(float64(h.spot.Y), 0))
    for i := yStart; i < h.spot.Y+h.spot.Height && i < height; i++ {
        start, stop := h.shape.intersections(h.spot, i)
        start = int(math.Max(float64(start), 0))
        for z := start; z <= stop && z < width; z++ {
            pix[i][z] = h.generatePixel()
        }
    }
}

func (h *hotspot) generatePixel() uint16 {
    if h.spot.MaxTemp <= h.spot.MinTemp {
        return uint16(h.spot.MinTemp)
    }

    return uint16(h.spot.MinTemp + rand.Intn(h.spot.MaxTemp-h.spot.MinTemp))
}

type spot struct {
    ShapeType string `json:"shapeType"`
    X         int    `json:"x"`
    Y         int    `json:"y"`
    Width     int    `json:"width"`
    Height    int    `json:"height"`
    MinTemp   int    `json:"minTemp"`
    MaxTemp   int    `json:"maxTemp"`
}

func (hs *hotspot) UnmarshalJSON(data []byte) error {
    var h spot
    if err := json.Unmarshal(data, &h); err != nil {
        return err
    }
    hs.spot = &h

    if h.ShapeType == "circle" {
        hs.shape = NewCircle(&h)
        return nil
    }
    hs.shape = rectangle{}

    return nil
}

func makeFrame(minTemp, maxTemp int, hotspots []hotspot) *cptvframe.Frame {
    var pix int
    out := cptvframe.NewFrame(camera)
    for y, row := range out.Pix {
        for x, _ := range row {
            if maxTemp <= minTemp {
                pix = minTemp
            } else {
                pix = minTemp + rand.Intn(maxTemp-minTemp)
            }
            out.Pix[y][x] = uint16(pix)
        }
    }
    addHotpsots(out.Pix, hotspots)
    return out
}

func addHotpsots(pix [][]uint16, hotspots []hotspot) {
    if hotspots == nil {
        return
    }

    for _, hotspot := range hotspots {
        hotspot.addSpot(pix)
    }
}

func setStatus(telemetry *cptvframe.Telemetry, timeon time.Duration, ffc bool, plusMS int) {
    telemetry.TimeOn = timeon + time.Duration(plusMS)*time.Millisecond
    if ffc {
        telemetry.FFCState = "FFCRunning"
        telemetry.LastFFCTime = timeon + time.Duration(plusMS)*time.Millisecond
    }
}
func sendCPTV(conn *net.UnixConn, params url.Values) error {
    file := params.Get("cptv-file")
    fullpath := path.Join(cptvDir, file)
    if _, err := os.Stat(fullpath); err != nil {
        log.Printf("%v does not exist\n", fullpath)
        return err
    }
    r, err := cptv.NewFileReader(fullpath)
    if err != nil {
        return err
    }
    defer r.Close()
    log.Printf("sending raw frames of %v\n", fullpath)
    return sendFramesFromFile(conn, r, params)

}

func getHotspots(params url.Values) ([]hotspot, error) {
    hotspotRaw := params.Get("hotspots")
    var hotspots []hotspot
    if hotspotRaw != "" {
        if err := json.Unmarshal([]byte(hotspotRaw), &hotspots); err != nil {
            log.Printf("Could not parse hotspot %v\n", err)
            return nil, err
        }
    }
    return hotspots, nil
}

func sendFrames(conn *net.UnixConn, params url.Values, frames int) error {
    minTemp, _ := strconv.Atoi(params.Get("minTemp"))
    maxTemp, _ := strconv.Atoi(params.Get("maxTemp"))
    ffc, _ := strconv.ParseBool(params.Get("ffc"))
    fps, _ := strconv.Atoi(params.Get("fps"))
    if fps == 0 {
        fps = camera.FPS()
    }
    if minTemp == 0 {
        minTemp = frameMinTemp
    }
    if maxTemp == 0 {
        maxTemp = frameMaxTemp
    }
    hotspots, _ := getHotspots(params)

    // Telemetry size of 640 -64(size of telemetry words)
    var reaminingBytes [576]byte
    frameSleep := time.Duration(1000/fps) * time.Millisecond
    for i := 0; i < frames; i++ {
        if stopSending {
            return nil
        }

        frame := makeFrame(minTemp, maxTemp, hotspots)
        setStatus(&frame.Status, time.Since(startTime), ffc, 0)

        buf := rawTelemetryBytes(frame.Status)
        _ = binary.Write(buf, binary.BigEndian, reaminingBytes)
        for _, row := range frame.Pix {
            for x, _ := range row {
                _ = binary.Write(buf, binary.BigEndian, row[x])
            }
        }
        // replicate cptv frame rate
        time.Sleep(frameSleep)
        if _, err := conn.Write(buf.Bytes()); err != nil {
            // reconnect to socket
            wg.Done()
            return err
        }

    }
    return nil
}

func sendFramesFromFile(conn *net.UnixConn, r *cptv.FileReader, params url.Values) error {
    fps, _ := strconv.Atoi(params.Get("fps"))
    if fps == 0 {
        fps = r.Reader.FPS()
        if fps == 0 {
            fps = camera.FPS()
        }
    }
    start, _ := strconv.Atoi(params.Get("start"))
    end, _ := strconv.Atoi(params.Get("end"))
    hotspots, _ := getHotspots(params)
    ffc, _ := strconv.ParseBool(params.Get("ffc"))

    frame := r.Reader.EmptyFrame()
    // Telemetry size of 640 -64(size of telemetry words)
    var reaminingBytes [576]byte

    frameSleep := time.Duration(1000/fps) * time.Millisecond
    for index := 0; index <= end || end == 0; index++ {
        if stopSending {
            return nil
        }
        err := r.ReadFrame(frame)
        if err == io.EOF {
            break
        }

        if index >= start {
            addHotpsots(frame.Pix, hotspots)
            setStatus(&frame.Status, frame.Status.TimeOn, ffc, 0)

            buf := rawTelemetryBytes(frame.Status)
            _ = binary.Write(buf, binary.BigEndian, reaminingBytes)
            for _, row := range frame.Pix {
                for x, _ := range row {
                    _ = binary.Write(buf, binary.BigEndian, row[x])
                }
            }
            // replicate cptv frame rate
            time.Sleep(frameSleep)
            if _, err := conn.Write(buf.Bytes()); err != nil {
                // reconnect to socket
                wg.Done()
                return err
            }
        }

    }
    return nil
}

func rawTelemetryBytes(t cptvframe.Telemetry) *bytes.Buffer {
    var tw telemetryWords
    tw.TimeOn = uint32(t.TimeOn.Milliseconds())
    tw.StatusBits = ffcStateToStatus(t.FFCState)
    tw.FrameCounter = uint32(t.FrameCount)
    tw.FrameMean = t.FrameMean
    tw.FPATemp = ToK(t.TempC)
    tw.FPATempLastFFC = ToK(t.LastFFCTempC)
    tw.TimeCounterLastFFC = uint32(t.LastFFCTime.Milliseconds())
    buf := new(bytes.Buffer)
    binary.Write(buf, lepton3.Big16, tw)
    return buf
}

const statusFFCStateShift uint32 = 4

func ffcStateToStatus(status string) uint32 {
    var state uint32 = 3
    switch status {
    case lepton3.FFCNever:
        state = 0
    case lepton3.FFCImminent:
        state = 1
    case lepton3.FFCRunning:
        state = 2
    }
    state = state << statusFFCStateShift
    return state
}

type centiK uint16

func ToK(c float64) centiK {
    return centiK(c*100 + 27315)
}

type telemetryWords struct {
    TelemetryRevision  uint16    // 0  *
    TimeOn             uint32    // 1  *
    StatusBits         uint32    // 3  * Bit field
    Reserved5          [8]uint16 // 5  *
    SoftwareRevision   uint64    // 13 - Junk.
    Reserved17         [3]uint16 // 17 *
    FrameCounter       uint32    // 20 *
    FrameMean          uint16    // 22 * The average value from the whole frame
    FPATempCounts      uint16    // 23
    FPATemp            centiK    // 24 *
    Reserved25         [4]uint16 // 25
    FPATempLastFFC     centiK    // 29
    TimeCounterLastFFC uint32    // 30 *
}
