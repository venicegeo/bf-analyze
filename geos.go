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
	"log"

	"github.com/paulsmith/gogeos/geos"
	"github.com/venicegeo/geojson-go/geojson"
)

const (
	// DETECTION is the key for the GeoJSON object to indicate whether a shoreline
	// was previously detected
	DETECTION = "detection"
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
		log.Printf("Unimplemented GeometryCollection")
	case geojson.MultiPolygon:
		log.Printf("Unimplemented MultiPolygon")
	case geojson.Feature:
		return toGeos(gt.Geometry)
	default:
		log.Printf("unexpected type %T\n", gt)
	}
	return geometry, err
}

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
		}
	}
	return result, err
}
func matchFeature(baselineFeature *geojson.Feature, detectedGeometries **geos.Geometry) error {
	var (
		err error
		baselineGeometry,
		currentGeometry *geos.Geometry
		disjoint = true
		count    int
	)
	baselineGeometry, err = toGeos(*baselineFeature)

	if err == nil {
		count, err = (*detectedGeometries).NGeometry()
		for inx := 0; inx < count; inx++ {
			currentGeometry, err = (*detectedGeometries).Geometry(inx)
			if err != nil {
				break
			}
			disjoint, err = baselineGeometry.Disjoint(currentGeometry)
			if err != nil {
				break
			}
			if !disjoint {
				// Since we have already matched this geometry, we won't need to try to match it again
				// Why doesn't this work?
				var tc1 int
				*detectedGeometries, err = (*detectedGeometries).Difference(currentGeometry)
				tc1, _ = (*detectedGeometries).NGeometry()
				log.Printf("O: %v; N: %v", count, tc1)
				break
			}
		}
	}
	if err == nil {
		if disjoint {
			var undetected = make(map[string]interface{})
			undetected[DETECTION] = "Undetected"
			baselineFeature.Properties = undetected
		} else {
			var detected = make(map[string]interface{})
			detected[DETECTION] = "Detected"
			gc := geojson.GeometryCollection{Type: geojson.GEOMETRYCOLLECTION}
			var (
				newGeometry interface{}
			)
			newGeometry, err = fromGeos(currentGeometry)
			if err == nil {
				slice := [...]interface{}{baselineFeature.Geometry, newGeometry}
				gc.Geometries = slice[:]
				baselineFeature.Geometry = gc
				baselineFeature.Properties = detected
			}
		}
	}
	return err
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

func mlsToMultiPolygon(input *geos.Geometry) (*geos.Geometry, error) {
	var (
		result     *geos.Geometry
		err        error
		innerRings []*geos.Geometry
		// polygons   []*geos.Geometry
		count      int
		lineString *geos.Geometry
		ring       *geos.Geometry
		envelope   *geos.Geometry
		polygon    *geos.Geometry
		closed     bool
	)
	envelope, err = input.Envelope()
	if err == nil {
		count, err = input.NGeometry()
		for inx := 0; inx < count; inx++ {
			lineString, err = input.Geometry(inx)
			if err != nil {
				break
			}
			closed, err = lineString.IsClosed()
			if err != nil {
				break
			}
			if closed {
				ring, err = linearRingFromLineString(lineString)
				if err != nil {
					break
				}
				innerRings = append(innerRings, ring)
			}
		}

		// Associate the inner rings with the right polygon
		if err == nil {
			ring, err = envelope.Shell()
			if err == nil {
				polygon, err = geos.PolygonFromGeom(ring, innerRings[:]...)
				// if err == nil {
				// 	// Later we may have multiple outer shells and will have to deal with them separately
				// 	polygons = append(polygons, polygon)
				// 	log.Printf("%v\n", polygons)
				// }
			}
		}
	}
	if err == nil {
		result, err = geos.NewCollection(geos.MULTIPOLYGON, polygon)
	}
	return result, err
}
