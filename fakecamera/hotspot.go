package fakecamera

import (
    "encoding/json"
    "math"
    "math/rand"
)

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
