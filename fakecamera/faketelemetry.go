package fakecamera

import (
	"bytes"
	"encoding/binary"

	cptvframe "github.com/TheCacophonyProject/go-cptv/cptvframe"
	lepton3 "github.com/TheCacophonyProject/lepton3"
)

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
