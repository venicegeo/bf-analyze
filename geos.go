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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/montanaflynn/stats"
	"github.com/paulsmith/gogeos/geos"
	"github.com/venicegeo/geojson-go/geojson"
)

func parseCoord(input []float64) geos.Coord {
	return geos.NewCoord(input[0], input[1])
}
func parseCoordArray(input [][]float64) []geos.Coord {
	var result []geos.Coord
	for inx := 0; inx < len(input); inx++ {
		result = append(result, parseCoord(input[inx]))
	}
	return result
}

// toGeos takes a GeoJSON object and returns a GEOS geometry
func toGeos(input interface{}) (*geos.Geometry, error) {
	var (
		geometry *geos.Geometry
		err      error
	)

	switch gt := input.(type) {
	case geojson.Point:
		geometry, err = geos.NewPoint(parseCoord(gt.Coordinates))
	case geojson.LineString:
		geometry, err = geos.NewLineString(parseCoordArray(gt.Coordinates)...)
	case geojson.Polygon:
		var coords []geos.Coord
		var coordsArray [][]geos.Coord
		for jnx := 0; jnx < len(gt.Coordinates); jnx++ {
			coords = parseCoordArray(gt.Coordinates[jnx])
			coordsArray = append(coordsArray, coords)
		}
		geometry, err = geos.NewPolygon(coordsArray[0], coordsArray[1:]...)
	case geojson.MultiPoint:
		var points []*geos.Geometry
		var point *geos.Geometry
		for jnx := 0; jnx < len(gt.Coordinates); jnx++ {
			point, err = geos.NewPoint(parseCoord(gt.Coordinates[jnx]))
			points = append(points, point)
		}
		geometry, err = geos.NewCollection(geos.MULTIPOINT, points...)
	case geojson.MultiLineString:
		var lineStrings []*geos.Geometry
		var lineString *geos.Geometry
		for jnx := 0; jnx < len(gt.Coordinates); jnx++ {
			lineString, err = geos.NewLineString(parseCoordArray(gt.Coordinates[jnx])...)
			lineStrings = append(lineStrings, lineString)
		}
		geometry, err = geos.NewCollection(geos.MULTILINESTRING, lineStrings...)

	case geojson.GeometryCollection:
		err = errors.New("Unimplemented GeometryCollection")
	case geojson.MultiPolygon:
		err = errors.New("Unimplemented MultiPolygon")
	case geojson.Feature:
		return toGeos(gt.Geometry)
	default:
		err = fmt.Errorf("Unexpected type %T\n", gt)
	}
	return geometry, err
}

// fromGeos takes a GEOS geometry and returns a GeoJSON object
func fromGeos(input *geos.Geometry) (interface{}, error) {
	var (
		result interface{}
		err    error
		gType  geos.GeometryType
		coords []geos.Coord
	)
	gType, err = input.Type()
	if err == nil {
		switch gType {
		case geos.LINESTRING:
			coords, err = input.Coords()
			if err == nil {
				var coordinates [][]float64
				for inx := 0; inx < len(coords); inx++ {
					arr := [...]float64{coords[inx].X, coords[inx].Y}
					coordinates = append(coordinates, arr[:])
				}
				result = geojson.LineString{Type: geojson.LINESTRING, Coordinates: coordinates}
			}
		default:
			err = fmt.Errorf("Unimplemented %T", gType)
		}

	}
	return result, err
}
func linearRingFromLineString(input *geos.Geometry) (*geos.Geometry, error) {
	var coords []geos.Coord
	var result *geos.Geometry
	var err error

	coords, err = input.Coords()
	if err == nil {
		result, err = geos.NewLinearRing(coords[:]...)
	}
	return result, err
}
func lineStringFromGeometry(input *geos.Geometry) (*geos.Geometry, error) {
	var (
		coords       []geos.Coord
		result       *geos.Geometry
		err          error
		geometryType geos.GeometryType
	)

	geometryType, err = input.Type()
	if err != nil {
		return result, err
	}
	switch geometryType {
	case geos.LINESTRING:
		result = input
	case geos.LINEARRING:
		coords, err = input.Coords()
		if err == nil {
			result, err = geos.NewLineString(coords[:]...)
		}
	case geos.POLYGON:
		var shell *geos.Geometry
		shell, err = input.Shell()
		if err == nil {
			// Reenter
			result, err = lineStringFromGeometry(shell)
		}
	default:
		err = fmt.Errorf("Cannot create a line string from type %v.", geometryType)
	}
	return result, err
}

// multiPolygonize turns a slice of LineStrings into a MultiPolygon
func multiPolygonize(input []*geos.Geometry) (*geos.Geometry, error) {
	var (
		result         *geos.Geometry
		mls            *geos.Geometry
		err            error
		geometryString string
		file           *os.File
	)

	// Take the input, turn it into a MultiLineString so we can pass it to C++-land
	mls, err = geos.NewCollection(geos.MULTILINESTRING, input[:]...)
	if err != nil {
		return nil, err
	}

	// Write the MLS to a temp file
	geometryString, err = mls.ToWKT()
	file, err = ioutil.TempFile("", "mls")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())

	file.Write([]byte(geometryString))

	// Call our other application, which returns WKT
	cmd := exec.Command("/Users/JeffYutzler/projects/venicegeo/bf-line-analyzer/bld/bf_la", "-mlp", file.Name())
	bytes, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	result, err = geos.FromWKT(string(bytes))

	return result, err
}

// mlsToMPoly takes a MultiLineString and turns it into a MultiPolygon
// This includes handling all of the interior (inner) rings
func mlsToMPoly(input *geos.Geometry) (*geos.Geometry, error) {
	var (
		result     *geos.Geometry
		err        error
		rings      []*geos.Geometry
		chords     []*geos.Geometry
		polygons   []*geos.Geometry
		count      int
		lineString *geos.Geometry
		ring       *geos.Geometry
		envelope   *geos.Geometry
		polygon    *geos.Geometry
		closed     bool
	)

	// Create two bins, one of rings and one of chords
	// The envelope itself is the first chord
	envelope, err = input.Envelope()
	if err != nil {
		return nil, err
	}
	ring, err = envelope.Shell()
	if err != nil {
		return nil, err
	}
	lineString, err = lineStringFromGeometry(ring)
	if err != nil {
		return nil, err
	}
	chords = append(chords, lineString)

	count, err = input.NGeometry()
	for inx := 0; inx < count; inx++ {
		lineString, err = input.Geometry(inx)
		if err != nil {
			return nil, err
		}
		closed, err = lineString.IsClosed()
		if err != nil {
			return nil, err
		}
		if closed {
			ring, err = linearRingFromLineString(lineString)
			if err != nil {
				return nil, err
			}
			rings = append(rings, ring)
		} else {
			chords = append(chords, lineString)
		}
	}

	// Create a MultiPolygon covering the AOI
	if len(chords) > 1 {
		result, err = multiPolygonize(chords)
	} else {
		result, err = geos.NewCollection(geos.MULTIPOLYGON, envelope)
	}
	if err != nil {
		return nil, err
	}

	// Make a new bag of polygons,
	// associating the detected rings with the right polygon
	count, err = result.NGeometry()
	if err != nil {
		return nil, err
	}
	for inx := 0; inx < count; inx++ {
		var (
			innerRings []*geos.Geometry
			contains   bool
		)
		polygon, err = result.Geometry(inx)
		if err != nil {
			return nil, err
		}
		for jnx := 0; jnx < len(rings); jnx++ {
			contains, err = polygon.Contains(rings[jnx])
			if err != nil {
				return nil, err
			}
			if contains {
				innerRings = append(innerRings, rings[jnx])
			}
		}
		ring, err = polygon.Shell()
		if err != nil {
			return nil, err
		}
		polygon, err = geos.PolygonFromGeom(ring, innerRings[:]...)
		if err != nil {
			return nil, err
		}
		polygons = append(polygons, polygon)
	}

	// Reconstruct the MultiPolygon
	result, err = geos.NewCollection(geos.MULTIPOLYGON, polygons[:]...)

	return result, err
}

func lineStringsToFloat64Data(first, second *geos.Geometry) (stats.Float64Data, error) {
	var (
		err          error
		coords       []geos.Coord
		data         []float64
		distance     float64
		point        *geos.Geometry
		geometryType geos.GeometryType
	)

	geometryType, err = first.Type()
	switch geometryType {
	case geos.LINESTRING:
		coords, err = first.Coords()
	case geos.POLYGON:
		first, _ = first.Shell()
		coords, err = first.Coords()
	}
	if err != nil {
		return nil, err
	}
	data = make([]float64, len(coords))
	for inx := range coords {
		point, err = geos.NewPoint(coords[inx])
		if err != nil {
			return nil, err
		}
		distance, err = point.Distance(second)
		if err != nil {
			return nil, err
		}
		data[inx] = distance
	}
	return stats.LoadRawData(data), err
}
