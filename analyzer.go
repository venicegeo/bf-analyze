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
	if baselineGeometry, err = toGeos(*baselineFeature); err != nil {
		return err
	}
	// And from GEOS to a GEOS LineString
	if baselineGeometry, err = lineStringFromGeometry(baselineGeometry); err != nil {
		return err
	}
	if baselineClosed, err = baselineGeometry.IsClosed(); err != nil {
		return err
	}
	if count, err = (*detectedGeometries).NGeometry(); err != nil {
		return err
	}
	for inx := 0; inx < count; inx++ {
		if detectedGeometry, err = (*detectedGeometries).Geometry(inx); err != nil {
			return err
		}

		// To be a match they must both have the same closedness...
		if detectedClosed, err = detectedGeometry.IsClosed(); err != nil {
			return err
		}
		if baselineClosed != detectedClosed {
			continue
		}

		// And somehow overlap each other (not be disjoint)...
		if disjoint, err = baselineGeometry.Disjoint(detectedGeometry); err != nil {
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
			if newGeometry, err = fromGeos(detectedGeometry); err != nil {
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
		matchedFeatures    []geojson.Feature
		geometry           *geos.Geometry
		err                error
		bytes              []byte
		count              int
		features           []geojson.Feature
		detectedGeometries *geos.Geometry
	)

	if features, err = baseline.Features(); err != nil {
		return err
	}

	if detectedGeometries, err = detected.MultiLineString(); err != nil {
		return err
	}

	// Try to match the geometry for each feature with what we detected
	for inx := range features {
		feature := features[inx]
		if err = matchFeature(&feature, &detectedGeometries); err != nil {
			return err
		}

		matchedFeatures = append(matchedFeatures, feature)
	}

	// Construct new features for the geometries that didn't match up
	var newDetection = make(map[string]interface{})
	newDetection[DETECTION] = "New Detection"
	if count, err = detectedGeometries.NGeometry(); err != nil {
		return err
	}
	for inx := 0; inx < count; inx++ {
		var gjGeometry interface{}
		if geometry, err = detectedGeometries.Geometry(inx); err != nil {
			return err
		}
		if gjGeometry, err = fromGeos(geometry); err != nil {
			return err
		}
		feature := geojson.Feature{Type: geojson.FEATURE,
			Geometry:   gjGeometry,
			Properties: newDetection}
		matchedFeatures = append(matchedFeatures, feature)
	}

	fc := geojson.NewFeatureCollection(matchedFeatures)
	bytes, err = json.Marshal(fc)

	if err == nil {
		fmt.Printf("%v\n", string(bytes))
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

	if geometries, err = scene.MultiLineString(); err != nil {
		return err
	}
	if mpolygon, err = mlsToMPoly(geometries); err != nil {
		return err
	}
	if count, err = mpolygon.NGeometry(); err != nil {
		return err
	}
	var polygonMetadatas = make([]polygonMetadata, count)

	for inx := 0; inx < count; inx++ {
		polygonMetadatas[inx].index = inx
		polygon, err = mpolygon.Geometry(inx)
		if err != nil {
			return err
		}
		// We need two areas for each component polygon
		// The total area (which considers holes)
		if polygonMetadatas[inx].totalArea, err = polygon.Area(); err != nil {
			return err
		}
		// The shell (boundary)
		if boundary, err = polygon.Shell(); err != nil {
			return err
		}
		if boundary, err = geos.PolygonFromGeom(boundary); err != nil {
			return err
		}
		if polygonMetadatas[inx].boundaryArea, err = boundary.Area(); err != nil {
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
			if polygon2, err = mpolygon.Geometry(jnx); err != nil {
				return err
			}
			if touches, err = polygon2.Touches(polygon); err != nil {
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
