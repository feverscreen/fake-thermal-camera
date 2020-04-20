// fake-lepton - read a cptv file and send it has raw frames to thermal-recorder
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
    lockChannel chan struct{} = make(chan struct{}, 1)
    wg          sync.WaitGroup
    cptvDir     = "/cptv-files"

    stopSending = false

    queueLock sync.Mutex
    sendQueue = make([]url.Values, 0, 3)
    camera    cptvframe.CameraSpec
)

// type argSpec struct {
//     CPTVDir   string `arg:"-c,--cptv-dir" help:"base path of cptv files"`
//     ConfigDir string `arg:"-c,--config" help:"path to configuration directory"`
// }

// func procArgs() argSpec {
//     args := argSpec{CPTVDir: cptvDir}
//     args.ConfigDir = goconfig.DefaultConfigDir

//     arg.MustParse(&args)
//     return args
// }

// func main() {
//     args := procArgs()

//     err := runMain()
//     fmt.Printf("closing")
//     if err != nil {
//         log.Fatal(err)
//     }

// }

// type fakeLepton struct {
//     CPTVDir   string
//     ConfigDir string
// }

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
    log.Printf("Listening for send cptv ...")
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
        clearQueue()
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
            frames := generateFrames(conn, params, repeat)
            sendFrames(conn, params, frames)
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

func clearQueue() {
    queueLock.Lock()
    defer queueLock.Unlock()
    sendQueue = make([]url.Values, 0, 3)
    forceStop()
}

type shape interface {
    inside(h *spot, x, y int) bool
}

type rectangle struct {
}

func (r rectangle) inside(h *spot, x, y int) bool {
    return (x >= h.X && x < h.X+h.Width) && (y >= h.Y && y < h.Y+h.Height)
}

type circle struct {
    originX float64
    originY float64
    xRadSq  float64
    yRadSq  float64
}

func (r circle) inside(h *spot, x, y int) bool {
    res := math.Pow(float64(x)-r.originX, 2) / r.xRadSq
    res += math.Pow(float64(y)-r.originY, 2) / r.yRadSq
    return res <= 1
}

type hotspot struct {
    spot  *spot
    shape shape
}

func (h *hotspot) generatePixel() int {
    return h.spot.MinTemp + rand.Intn(h.spot.MaxTemp-h.spot.MinTemp)
}

func (h *hotspot) inside(x, y int) bool {
    return h.shape.inside(h.spot, x, y)
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
        c := &circle{}
        c.originX = float64(h.X) + float64(h.Width)/2.0
        c.originY = float64(h.Y) + float64(h.Height)/2.0
        c.xRadSq = math.Pow(float64(h.Width)/2.0, 2)
        c.yRadSq = math.Pow(float64(h.Height)/2.0, 2)
        hs.shape = c
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
            pix = minTemp + rand.Intn(maxTemp-minTemp)

            for _, hotspot := range hotspots {
                if hotspot.inside(x, y) {
                    pix = hotspot.generatePixel()
                    break
                }

            }
            out.Pix[y][x] = uint16(pix)
        }
    }

    return out
}

func generateFrames(conn *net.UnixConn, params url.Values, repeat int) []*cptvframe.Frame {
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

    hotspotRaw := params.Get("hotspots")

    var hotspots []hotspot
    if hotspotRaw != "" {
        if err := json.Unmarshal([]byte(hotspotRaw), &hotspots); err != nil {
            log.Printf("Coult not parse hotspot %v\n", err)
        }
    }

    frames := make([]*cptvframe.Frame, repeat)
    for i := 0; i < repeat; i++ {
        frame := makeFrame(minTemp, maxTemp, hotspots)
        if ffc {
            frame.StatusFFCState = "FFCRunning"
            frame.Status.TimeOn = time.Duration(i*fps) * time.Millisecond
            frame.Status.LastFFCTime = time.Duration(i*fps) * time.Millisecond
        }
        frames[i] = frame
    }
    return frames
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

func sendFrames(conn *net.UnixConn, params url.Values, frames []*cptvframe.Frame) error {
    fps, _ := strconv.Atoi(params.Get("fps"))
    if fps == 0 {
        fps = camera.FPS()
    }

    // Telemetry size of 640 -64(size of telemetry words)
    var reaminingBytes [576]byte

    frameSleep := time.Duration(1000/fps) * time.Millisecond
    for _, frame := range frames {
        if stopSending {
            return nil
        }
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
    tw.TimeOn = ToMS(t.TimeOn)
    tw.StatusBits = ffcStateToStatus(t.FFCState)
    tw.FrameCounter = uint32(t.FrameCount)
    tw.FrameMean = t.FrameMean
    tw.FPATemp = ToK(t.TempC)
    tw.FPATempLastFFC = ToK(t.LastFFCTempC)
    tw.TimeCounterLastFFC = ToMS(t.LastFFCTime)
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

type durationMS uint32
type centiK uint16

func ToK(c float64) centiK {
    return centiK(c*100 + 27315)
}

func ToMS(d time.Duration) durationMS {
    return durationMS(d / time.Millisecond)
}

type telemetryWords struct {
    TelemetryRevision  uint16     // 0  *
    TimeOn             durationMS // 1  *
    StatusBits         uint32     // 3  * Bit field
    Reserved5          [8]uint16  // 5  *
    SoftwareRevision   uint64     // 13 - Junk.
    Reserved17         [3]uint16  // 17 *
    FrameCounter       uint32     // 20 *
    FrameMean          uint16     // 22 * The average value from the whole frame
    FPATempCounts      uint16     // 23
    FPATemp            centiK     // 24 *
    Reserved25         [4]uint16  // 25
    FPATempLastFFC     centiK     // 29
    TimeCounterLastFFC durationMS // 30 *
}
