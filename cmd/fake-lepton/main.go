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

package main

import (
    "bytes"
    "encoding/binary"
    "errors"
    "fmt"
    "gopkg.in/yaml.v1"
    "io"
    "log"
    "net"
    "net/url"
    "os"
    "path"
    "strconv"
    "sync"
    "time"

    arg "github.com/alexflint/go-arg"

    "github.com/TheCacophonyProject/go-cptv"
    cptvframe "github.com/TheCacophonyProject/go-cptv/cptvframe"
    lepton3 "github.com/TheCacophonyProject/lepton3"
    "github.com/TheCacophonyProject/thermal-recorder/headers"
)

const (
    sendSocket = "/var/run/lepton-frames"
    framesHz   = 9
    queueSleep = 1 * time.Second

    sleepTime   = 30 * time.Second
    lockTimeout = 10 * time.Second
)

var (
    lockChannel chan struct{} = make(chan struct{}, 1)
    wg          sync.WaitGroup
    cptvDir     = "/cptv-files"

    stopSending = make(chan bool)

    queueLock sync.Mutex
    sendQueue = make([]url.Values, 3)
)

type argSpec struct {
    CPTVDir string `arg:"-c,--cptv-dir" help:"base path of cptv files"`
}

func procArgs() argSpec {
    args := argSpec{CPTVDir: cptvDir}
    arg.MustParse(&args)
    return args
}

func main() {
    err := runMain()
    fmt.Printf("closing")
    if err != nil {
        log.Fatal(err)
    }

}

func runMain() error {
    args := procArgs()
    cptvDir = args.CPTVDir

    log.Println("starting d-bus service")
    dbusService, err := startService(nil)
    if err != nil {
        return err
    }

    for {
        err := connectToSocket(dbusService)
        if err != nil {
            fmt.Printf("Could not connect to socket %v will try again in %v\n", err, sleepTime)
            time.Sleep(sleepTime)
        } else {
            fmt.Print("Disconnected\n")
        }
    }
}

func connectToSocket(dbusService *service) error {
    log.Printf("dialing frame output socket %s\n", sendSocket)
    conn, err := net.DialUnix("unix", nil, &net.UnixAddr{
        Net:  "unix",
        Name: sendSocket,
    })
    if err != nil {
        fmt.Printf("error %v\n", err)
        return errors.New("error: connecting to frame output socket failed")
    }
    defer conn.Close()
    conn.SetWriteBuffer(lepton3.FrameCols * lepton3.FrameCols * 2 * 20)
    dbusService.conn = conn

    camera_specs := map[string]interface{}{
        headers.YResolution: lepton3.FrameRows,
        headers.XResolution: lepton3.FrameCols,
        headers.FrameSize:   lepton3.BytesPerFrame,
        headers.Model:       lepton3.Model,
        headers.Brand:       lepton3.Brand,
        headers.FPS:         framesHz,
    }

    cameraYAML, err := yaml.Marshal(camera_specs)
    if _, err := conn.Write(cameraYAML); err != nil {
        return err
    }

    conn.Write([]byte("\n"))
    fmt.Printf("Listening for send cptv ...")
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

func send(params url.Values) {
    enq, err := strconv.ParseBool(params.Get("enqueue"))
    if err != nil {
        enq = false
    }
    if enq {
        clearQueue()
    }
    enqueue(params)
}

func queueLoop(conn *net.UnixConn) error {
    for {
        params := dequeue()
        if params == nil {
            time.Sleep(queueSleep)
            continue
        }
        repeat, _ := strconv.Atoi(params.Get("repeat"))
        if repeat == 0 {
            repeat = 1
        }

        for i := 0; i < repeat; i++ {
            err := sendCPTV(conn, params)
            if err != nil {
                return err
            }
        }
    }
}

func forceStop() {
    stopSending <- true
}

func clearQueue() {
    queueLock.Lock()
    defer queueLock.Unlock()
    sendQueue = make([]url.Values, 3)
    forceStop()
}

func sendCPTV(conn *net.UnixConn, params url.Values) error {
    file := params.Get("filename")
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
    return sendFrames(conn, r, params)

}

func sendFrames(conn *net.UnixConn, r *cptv.FileReader, params url.Values) error {
    fps, _ := strconv.Atoi(params.Get("fps"))
    if fps == 0 {
        fps := r.Reader.FPS()
        if fps == 0 {
            fps = framesHz
        }
    }

    frame := r.Reader.EmptyFrame()
    // Telemetry size of 640 -64(size of telemetry words)
    var reaminingBytes [576]byte

    frameSleep := time.Duration(1000/fps) * time.Millisecond
    for {
        select {
        case <-stopSending:
            // reset
            stopSending <- true
            return nil
        default:
            err := r.ReadFrame(frame)
            if err == io.EOF {
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
