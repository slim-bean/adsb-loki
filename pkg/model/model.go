package model

import (
	"encoding/json"
)

type Details struct {
	Registration *string `json:"registration,omitempty"`
	TypeCode     *string `json:"type_code,omitempty"`
	Military     *bool   `json:"military,omitempty"`
	Interesting  *bool   `json:"interesting,omitempty"`
	PIA          *bool   `json:"pia,omitempty"`
	LADD         *bool   `json:"ladd,omitempty"`
	Description  *string `json:"description,omitempty"`
	Manufactured *string `json:"manufactured,omitempty"`
	Owner        *string `json:"owner,omitempty"`
}

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

	// This field might be a number, a string (usually "ground"), or nil
	BarometerAltitude json.Token `json:"alt_baro,omitempty"`

	Details
}
