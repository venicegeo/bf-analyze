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
	"encoding/json"
	"fmt"
	"log"

	"github.com/montanaflynn/stats"
	"github.com/paulsmith/gogeos/geos"
	"github.com/venicegeo/geojson-go/geojson"
)

const (
	// DETECTION is the key for the GeoJSON property indicating whether a shoreline
	// was previously detected
	DETECTION = "detection"
	// DETECTEDSTATS is the key for the GeoJSON property indicating the variance
	// between the detected points and baseline linestring
	DETECTEDSTATS = "detected_stats"
	// BASELINESTATS is the key for the GeoJSON property indicating the variance
	// between the baseline points and detected linestring
	BASELINESTATS = "baseline_stats"
)

// matchFeature looks for geometries that match the given feature
// If a match is found, the feature is updated and the geometry is removed from the input collection
func matchFeature(baselineFeature *geojson.Feature, detectedGeometries **geos.Geometry) error {
	var (
		err error
		baselineGeometry,
		detectedGeometry *geos.Geometry
		disjoint       bool
		count          int
		baselineClosed bool
		detectedClosed bool
	)
	// Go from GeoJSON to GEOS
	baselineGeometry, err = toGeos(*baselineFeature)
	if err != nil {
		return err
	}
	// And from GEOS to a GEOS LineString
	baselineGeometry, err = lineStringFromGeometry(baselineGeometry)
	if err != nil {
		return err
	}
	baselineClosed, err = baselineGeometry.IsClosed()
	if err != nil {
		return err
	}
	count, err = (*detectedGeometries).NGeometry()
	for inx := 0; inx < count; inx++ {
		detectedGeometry, err = (*detectedGeometries).Geometry(inx)
		if err != nil {
			return err
		}

		// To be a match they must both have the same closedness...
		detectedClosed, err = detectedGeometry.IsClosed()
		if err != nil {
			return err
		}
		if baselineClosed != detectedClosed {
			continue
		}

		// And somehow overlap each other (not be disjoint)...
		disjoint, err = baselineGeometry.Disjoint(detectedGeometry)
		if err != nil {
			return err
		}

		if !disjoint {
			// Now that we have a match
			// Add some metadata regarding the match
			var (
				newGeometry interface{}
				detected    = make(map[string]interface{})
				data        stats.Float64Data
			)
			detected[DETECTION] = "Detected"
			if data, err = lineStringsToFloat64Data(detectedGeometry, baselineGeometry); err != nil {
				return err
			}
			if detected[DETECTEDSTATS], err = populateStatistics(data); err != nil {
				return err
			}
			if data, err = lineStringsToFloat64Data(baselineGeometry, detectedGeometry); err != nil {
				return err
			}
			if detected[BASELINESTATS], err = populateStatistics(data); err != nil {
				return err
			}

			// And replace the geometry with a GeometryCollection [baseline, detected]
			gc := geojson.GeometryCollection{Type: geojson.GEOMETRYCOLLECTION}
			newGeometry, err = fromGeos(detectedGeometry)
			if err != nil {
				return err
			}
			slice := [...]interface{}{baselineFeature.Geometry, newGeometry}
			gc.Geometries = slice[:]
			baselineFeature.Geometry = gc
			baselineFeature.Properties = detected

			// Since we have already found a match for this geometry
			// we won't need to try to match it again later so remove it from the list
			*detectedGeometries, err = (*detectedGeometries).Difference(detectedGeometry)
			return err
		}
	}

	// If we got here, there was no match
	var undetected = make(map[string]interface{})
	undetected[DETECTION] = "Undetected"
	baselineFeature.Properties = undetected
	return err
}
func qualitativeReview(detected Scene, baseline Scene) error {
	var (
		matchedFeatures []geojson.Feature
		geometry        *geos.Geometry
		err             error
		bytes           []byte
		count           int
	)

	features, err := baseline.Features()
	if err != nil {
		return err
	}

	detectedGeometries, err := detected.MultiLineString()
	if err != nil {
		return err
	}

	// Try to match the geometry for each feature with what we detected
	for inx := 0; inx < len(features); inx++ {
		feature := features[inx]
		err = matchFeature(&feature, &detectedGeometries)
		if err != nil {
			return err
		}

		matchedFeatures = append(matchedFeatures, feature)
	}

	// Construct new features for the geometries that didn't match up
	var newDetection = make(map[string]interface{})
	newDetection[DETECTION] = "New Detection"
	count, err = detectedGeometries.NGeometry()
	for inx := 0; inx < count; inx++ {
		var gjGeometry interface{}
		geometry, err = detectedGeometries.Geometry(inx)
		if err != nil {
			return err
		}
		gjGeometry, err = fromGeos(geometry)
		if err != nil {
			return err
		}
		feature := geojson.Feature{Type: geojson.FEATURE,
			Geometry:   gjGeometry,
			Properties: newDetection}
		matchedFeatures = append(matchedFeatures, feature)
	}

	if err == nil {
		fc := geojson.NewFeatureCollection(matchedFeatures)
		bytes, err = json.Marshal(fc)
		if err == nil {
			fmt.Printf("%v\n", string(bytes))
		}
	}
	return err
}

type polygonMetadata struct {
	boundaryArea, totalArea float64
	terminal                bool
	link                    *polygonMetadata
	index                   int
}

func populateStatistics(input stats.Float64Data) (map[string]interface{}, error) {
	var (
		result = make(map[string]interface{})
		err    error
	)
	if result["mean"], err = input.Mean(); err != nil {
		return result, err
	}
	result["median"], err = input.Median()
	return result, err
}

func quantitativeReview(scene Scene, envelope *geos.Geometry) error {
	var (
		err          error
		polygon      *geos.Geometry
		polygon2     *geos.Geometry
		mpolygon     *geos.Geometry
		boundary     *geos.Geometry
		geometries   *geos.Geometry
		count        int
		touches      bool
		positiveArea float64
		negativeArea float64
	)

	geometries, err = scene.MultiLineString()
	if err != nil {
		return err
	}

	mpolygon, err = mlsToMPoly(geometries)
	if err != nil {
		return err
	}
	count, err = mpolygon.NGeometry()
	var polygonMetadatas = make([]polygonMetadata, count)

	for inx := 0; inx < count; inx++ {
		polygonMetadatas[inx].index = inx
		polygon, err = mpolygon.Geometry(inx)
		if err != nil {
			return err
		}
		// We need two areas for each component polygon
		// The total area (which considers holes)
		polygonMetadatas[inx].totalArea, err = polygon.Area()
		if err != nil {
			return err
		}
		// The shell (boundary)
		boundary, err = polygon.Shell()
		if err != nil {
			return err
		}
		boundary, err = geos.PolygonFromGeom(boundary)
		if err != nil {
			return err
		}
		polygonMetadatas[inx].boundaryArea, err = boundary.Area()
		if err != nil {
			return err
		}

		// Construct an ordered acyclical graph of spaces,
		// with the first polygon being the terminal node
		if inx == 0 {
			polygonMetadatas[inx].terminal = true
		}
		// Iterate through all of the polygons
		for jnx := 1; jnx < count; jnx++ {
			// If a polygon is not already linked
			if (inx == jnx) || (polygonMetadatas[jnx].link != nil) {
				continue
			}
			polygon2, err = mpolygon.Geometry(jnx)
			if err != nil {
				return err
			}
			touches, err = polygon2.Touches(polygon)
			if err != nil {
				return err
			}
			// And it touches the current polygon, register the link
			if touches {
				polygonMetadatas[jnx].link = &(polygonMetadatas[inx])
			}
		}
	}
	for inx := 0; inx < count; inx++ {
		counter := 0
		// Count the steps to get from the current polygon to the terminal one
		// to determine its polarity
		for current := inx; !polygonMetadatas[current].terminal; {
			current = polygonMetadatas[current].link.index
			counter++
		}
		switch counter % 2 {
		case 0:
			positiveArea += polygonMetadatas[inx].totalArea
			negativeArea += polygonMetadatas[inx].boundaryArea - polygonMetadatas[inx].totalArea
		case 1:
			negativeArea += polygonMetadatas[inx].totalArea
			positiveArea += polygonMetadatas[inx].boundaryArea - polygonMetadatas[inx].totalArea
		}
	}
	log.Printf("+:%v -:%v Sum: %v Total:%v\n", positiveArea, negativeArea, positiveArea-negativeArea, positiveArea+negativeArea)
	return err
}
func quantitativeReview2(detectedGeometries *geos.Geometry, baselineGeometries *geos.Geometry) error {
	// walk through
	return nil
}
