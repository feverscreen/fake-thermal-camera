package fakecamera

import (
	"encoding/json"
	"log"
	"net/url"
	"strconv"
)

type params struct {
	url.Values
}

func (p *params) cptvFile() string {
	return p.Get("cptv-file")
}

func (p *params) minTemp() int {
	value, _ := strconv.Atoi(p.Get("minTemp"))
	return value
}

func (p *params) maxTemp() int {
	value, _ := strconv.Atoi(p.Get("maxTemp"))
	return value
}

func (p *params) ffc() bool {
	value, _ := strconv.ParseBool(p.Get("ffc"))
	return value
}

func (p *params) fps() int {
	value, _ := strconv.Atoi(p.Get("fps"))
	return value
}

func (p *params) lastFFC() int {
	value, err := strconv.Atoi(p.Get("ffc-time"))
	if err != nil {
		return -1
	}
	return value
}

func (p *params) enqueue() bool {
	value, _ := strconv.ParseBool(p.Get("enqueue"))
	return value
}

func (p *params) hotspots() []hotspot {
	hotspotRaw := p.Get("hotspots")
	var hotspots []hotspot
	if hotspotRaw != "" {
		if err := json.Unmarshal([]byte(hotspotRaw), &hotspots); err != nil {
			log.Printf("Could not parse hotspot %v\n", err)
			return nil
		}
	}
	return hotspots
}

func (p *params) repeat() int {
	value, _ := strconv.Atoi(p.Get("repeat"))
	if 1 > value {
		return 1
	}
	return value
}

func (p *params) generate() bool {
	value, _ := strconv.ParseBool(p.Get("generate"))
	return value
}
func (p *params) start() int {
	value, _ := strconv.Atoi(p.Get("start"))
	return value
}

func (p *params) end() int {
	value, _ := strconv.Atoi(p.Get("end"))
	return value
}
