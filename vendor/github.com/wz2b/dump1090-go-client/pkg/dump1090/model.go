package dump1090

import "encoding/json"

type Report struct {
	Now      float64    `json:"now"`
	Messages uint64     `json:"messages"`
	Aircraft []Aircraft `json:"aircraft"`
}

type Aircraft struct {
	Hex               string   `json:"hex"`
	Squawk            *string  `json:"squawk,omitempty"`
	Lat               *float64 `json:"lat,omitempty"`
	Lon               *float64 `json:"lon,omitempty"`
	Flight            *string  `json:"flight,omitempty"`
	GroundSpeed       *float64 `json:"gs,omitempty"`
	Track             *float64 `json:"track,omitempty"`
	Emergency         *string  `json:"emergency,omitempty"`
	Category          *string  `json:"category,omitempty"`
	Rssi              *float32 `json:"rssi,omitempty"`
	GeometricAltitude *float64 `json:"alt_geom,omitempty"`

	/*
	 * This field might be a number, a string (usually "ground"), or nil
	 */
	BarometerAltitude json.Token `json:"alt_baro,omitempty"`

	/*
	 * These fields are NOT part of the dump1090 format, but they may
	 * be added by external decorators
	 */
	Registration *string
	Description  *string
	AircraftType *string
}
