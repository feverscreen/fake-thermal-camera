package fakecamera

import (
	"io"
	"log"
	"os"
	"path"
	"time"

	"github.com/TheCacophonyProject/go-cptv"
	cptvframe "github.com/TheCacophonyProject/go-cptv/cptvframe"
)

// simple interface so we can read from a file or generate frames seemlessly
type frameReader interface {
	Next() (*cptvframe.Frame, error)
	FPS() int
	Close()
}

type frameMaker struct {
	frameReader
	hotspots []hotspot
	fps      int
	ffc      bool
	lastFFC  int
}

func NewFrameMaker(p *params) (*frameMaker, error) {
	var reader frameReader
	if p.generate() {
		reader = NewFakeReader(p)
	} else {
		var err error
		reader, err = NewCPTVReader(p)
		if err != nil {
			return nil, err
		}
	}

	fps := p.fps()
	if fps == 0 {
		fps = reader.FPS()
		if fps == 0 {
			fps = camera.FPS()
		}
	}
	return &frameMaker{frameReader: reader, hotspots: p.hotspots(), fps: fps, ffc: p.ffc(), lastFFC: p.lastFFC()}, nil
}

func addHotspots(pix [][]uint16, hotspots []hotspot) {
	if hotspots == nil {
		return
	}

	for _, hotspot := range hotspots {
		hotspot.addSpot(pix)
	}
}

func setStatus(telemetry *cptvframe.Telemetry, timeon time.Duration, ffc bool, plusMS int, lastFFC int) {
	telemetry.TimeOn = timeon + time.Duration(plusMS)*time.Millisecond
	if ffc {
		telemetry.FFCState = "FFCRunning"
		telemetry.LastFFCTime = timeon + time.Duration(plusMS)*time.Millisecond
	}
	if lastFFC >= 0 {
		telemetry.LastFFCTime = time.Duration(lastFFC) * time.Second
	}
}

// gets the next frame or returns an error if there are no more frames
// also adds hotspots and makes required changes to telemetry data
func (f *frameMaker) NextFrame() (*cptvframe.Frame, error) {
	frame, err := f.Next()
	if err != nil {
		return nil, err
	}

	addHotspots(frame.Pix, f.hotspots)
	setStatus(&frame.Status, time.Since(startTime), f.ffc, 0, f.lastFFC)
	return frame, err
}

type fakeReader struct {
	frame     *cptvframe.Frame
	minTemp   int
	maxTemp   int
	frames    int
	generated int
	fps       int
}

func NewFakeReader(p *params) *fakeReader {
	return &fakeReader{frame: cptvframe.NewFrame(camera), minTemp: p.minTemp(), maxTemp: p.maxTemp(), frames: p.repeat()}
}

func (f *fakeReader) Close() {
}

func (f *fakeReader) FPS() int {
	return 0
}

func (f *fakeReader) Next() (*cptvframe.Frame, error) {
	if f.generated >= f.frames {
		return nil, io.EOF
	}
	f.makeFrame()
	f.generated++
	return f.frame, nil
}

func (f *fakeReader) makeFrame() {
	for y, row := range f.frame.Pix {
		for x, _ := range row {
			f.frame.Pix[y][x] = generatePixel(f.minTemp, f.maxTemp)
		}
	}
}

type cptvReader struct {
	*cptv.FileReader
	frame    *cptvframe.Frame
	repeat   int
	frameNum int
	played   int
	start    int
	stop     int
	filepath string
}

func NewCPTVReader(params *params) (*cptvReader, error) {
	file := params.cptvFile()
	fullpath := path.Join(cptvDir, file)
	if _, err := os.Stat(fullpath); err != nil {
		log.Printf("%v does not exist\n", fullpath)
		return nil, err
	}

	f := &cptvReader{
		start:    params.start(),
		stop:     params.end(),
		repeat:   params.repeat(),
		filepath: fullpath,
	}
	err := f.newReader()
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (f *cptvReader) newReader() error {
	f.frameNum = 0
	r, err := cptv.NewFileReader(f.filepath)
	if err != nil {
		return err
	}
	f.FileReader = r
	f.frame = f.Reader.EmptyFrame()
	return f.readToStart()
}

func (f *cptvReader) readToStart() error {
	for f.frameNum < f.start {
		err := f.ReadFrame(f.frame)
		f.frameNum += 1
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *cptvReader) Next() (*cptvframe.Frame, error) {
	var err error
	if f.stop == 0 || f.frameNum <= f.stop {

		err = f.ReadFrame(f.frame)
		f.frameNum += 1
	} else {
		err = io.EOF
	}

	if err == io.EOF {

		f.played += 1
		if f.played >= f.repeat {
			return nil, io.EOF
		} else {
			//  play again
			err := f.newReader()
			if err != nil {
				return nil, err
			}
			return f.Next()
		}
	}
	return f.frame, err
}

func (f *cptvReader) fps() int {
	return f.Reader.FPS()
}
