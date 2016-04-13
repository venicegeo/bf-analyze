/*
Copyright 2016, RadiantBlue Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"io/ioutil"

	"github.com/paulsmith/gogeos/geos"
	"github.com/venicegeo/geojson-go/geojson"
)

// Scene is a shoreline scene, consisting of linework for shoreline features
type Scene struct {
	geoJSON         interface{}
	multiLineString *geos.Geometry
}

func parseGeoJSONFile(filename string) (Scene, error) {
	var result Scene
	bytes, err := ioutil.ReadFile(filename)
	if err == nil {
		result.geoJSON, err = geojson.Parse(bytes)
	}
	return result, err
}

// MultiLineString creates a geos.MultiLineString from the input and joins
// individual LineStrings together
func (s Scene) MultiLineString() (*geos.Geometry, error) {
	if s.multiLineString != nil {
		return s.multiLineString, nil
	}
	var (
		geometry *geos.Geometry
		result   *geos.Geometry
		err      error
	)

	result, err = geos.NewCollection(geos.MULTILINESTRING)

	// Pluck the geometries into an array
	detectedGJGeometries := geojson.ToGeometryArray(s.geoJSON)

	for inx := 0; inx < len(detectedGJGeometries); inx++ {
		// Transform the GeoJSON to a GEOS Geometry
		geometry, err = toGeos(detectedGJGeometries[inx])
		if err == nil {
			// If we get a polygon, we really just want its outer ring for now
			ttype, _ := geometry.Type()
			if ttype == geos.POLYGON {
				geometry, err = geometry.Shell()
			}
			if err == nil {
				result, err = result.Union(geometry)
			}
		}
		if err != nil {
			break
		}
	}

	if err == nil {
		// Join the geometries when possible
		result, err = result.LineMerge()
	}
	if err == nil {
		s.multiLineString = result
	}
	return s.multiLineString, err
}

// Features returns the GeoJSON Features
func (s Scene) Features() ([]geojson.Feature, error) {

	switch fc := s.geoJSON.(type) {
	case geojson.FeatureCollection:
		return fc.Features, nil
	default:
		return nil, errors.New("GeoJSON input must be a FeatureCollection.")
	}
}

func (s Scene) envelope() (*geos.Geometry, error) {
	result, err := s.MultiLineString()
	if err != nil {
		return nil, err
	}
	return result.Envelope()
}

func (s Scene) clip(input Scene) error {
	var geometry *geos.Geometry
	envelope, err := input.envelope()
	if err != nil {
		return err
	}
	geometry, err = s.MultiLineString()
	if err != nil {
		return err
	}
	geometry, err = envelope.Intersection(geometry)
	if err != nil {
		return err
	}
	s.multiLineString = geometry
	return err
}
