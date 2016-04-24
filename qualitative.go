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
	"github.com/montanaflynn/stats"
	"github.com/paulsmith/gogeos/geos"
	"github.com/venicegeo/geojson-go/geojson"
)

const (
	// DETECTION is the key for the GeoJSON property indicating whether a shoreline
	// was previously detected
	DETECTION = "detection"
	// DETECTEDSTATS is the key for the GeoJSON property containing the statistics
	// of the variance between the detected points and baseline linestring
	DETECTEDSTATS = "detected_stats"
	// BASELINESTATS is the key for the GeoJSON property containing the statistics
	// of the variance between the detected points and baseline linestring
	BASELINESTATS = "baseline_stats"
	// DETECTIONBIAS is the key for the GeoJSON property indicating the bias
	// detected between the detected and baseline features
	DETECTIONBIAS = "detection_bias"
)

func measureDisplacement(baseline, detected *geos.Geometry) (map[string]interface{}, error) {
	var (
		northingBias float64
		eastingBias  float64
		bcy          float64 // baseline centroid northing
		bcx          float64 // baseline centroid easting
		dcy          float64 // detected centroid northing
		dcx          float64 // detected centroid easting
		err          error
		data         stats.Float64Data
		biasMap      = make(map[string]interface{})
	)
	if bcx, bcy, err = centroidCoordsXY(baseline); err != nil {
		return nil, err
	}
	if dcx, dcy, err = centroidCoordsXY(detected); err != nil {
		return nil, err
	}
	northingBias = dcy - bcy
	eastingBias = dcx - bcx
	biasMap["northing"] = northingBias
	biasMap["easting"] = eastingBias

	// Correct for bias by displacing in the opposite direction
	if detected, err = displace(detected, -eastingBias, -northingBias); err != nil {
		return nil, err
	}
	if data, err = lineStringsToFloat64Data(detected, baseline); err != nil {
		return nil, err
	}
	if biasMap[DETECTEDSTATS], err = populateStatistics(data); err != nil {
		return nil, err
	}
	if data, err = lineStringsToFloat64Data(baseline, detected); err != nil {
		return nil, err
	}
	if biasMap[BASELINESTATS], err = populateStatistics(data); err != nil {
		return nil, err
	}
	return biasMap, nil
}

// matchFeature looks for geometries that match the given feature
// If a match is found, a composite feature is created and the geometry is removed from the input collection
// If no match is found, the feature is copied and the new copy gets updated properties
func matchFeature(baselineFeature *geojson.Feature, detectedGeometries **geos.Geometry) (*geojson.Feature, error) {
	var (
		err error
		baselineGeometry,
		detectedGeometry *geos.Geometry
		disjoint       bool
		count          int
		baselineClosed bool
		detectedClosed bool
		result         *geojson.Feature
	)
	// Go from GeoJSON to GEOS
	if baselineGeometry, err = toGeos(baselineFeature); err != nil {
		return result, err
	}
	// And from GEOS to a GEOS LineString
	if baselineGeometry, err = lineStringFromGeometry(baselineGeometry); err != nil {
		return result, err
	}
	if baselineClosed, err = baselineGeometry.IsClosed(); err != nil {
		return result, err
	}
	if count, err = (*detectedGeometries).NGeometry(); err != nil {
		return result, err
	}
	for inx := 0; inx < count; inx++ {
		if detectedGeometry, err = (*detectedGeometries).Geometry(inx); err != nil {
			return result, err
		}

		// To be a match they must both have the same closedness...
		if detectedClosed, err = detectedGeometry.IsClosed(); err != nil {
			return result, err
		}
		if baselineClosed != detectedClosed {
			continue
		}

		// And somehow overlap each other (not be disjoint)...
		if disjoint, err = baselineGeometry.Disjoint(detectedGeometry); err != nil {
			return result, err
		}

		if !disjoint {
			// Now that we have a match
			// Add some metadata regarding the match
			var (
				detectedGeojson interface{}
				detected        = make(map[string]interface{})
				data            stats.Float64Data
			)
			detected[DETECTION] = "Detected"
			if data, err = lineStringsToFloat64Data(detectedGeometry, baselineGeometry); err != nil {
				return result, err
			}
			if detected[DETECTEDSTATS], err = populateStatistics(data); err != nil {
				return result, err
			}
			if data, err = lineStringsToFloat64Data(baselineGeometry, detectedGeometry); err != nil {
				return result, err
			}
			if detected[BASELINESTATS], err = populateStatistics(data); err != nil {
				return result, err
			}

			if detected[DETECTIONBIAS], err = measureDisplacement(baselineGeometry, detectedGeometry); err != nil {
				return result, err
			}

			// Create a new geometry as a GeometryCollection [baseline, detected]
			if detectedGeojson, err = fromGeos(detectedGeometry); err != nil {
				return result, err
			}
			slice := [...]interface{}{baselineFeature.Geometry, detectedGeojson}
			result = geojson.NewFeature(geojson.NewGeometryCollection(slice[:]), "", detected)

			// Since we have already found a match for this geometry
			// we won't need to try to match it again later so remove it from the list
			*detectedGeometries, err = (*detectedGeometries).Difference(detectedGeometry)
			return result, err
		}
	}

	// If we got here, there was no match
	var undetected = make(map[string]interface{})
	undetected[DETECTION] = "Undetected"
	result = geojson.NewFeature(baselineFeature.Geometry, "", undetected)
	return result, err
}
func qualitativeReview(detected Scene, baseline Scene) (*geojson.FeatureCollection, error) {
	var (
		matchedFeatures    []*geojson.Feature
		geometry           *geos.Geometry
		err                error
		count              int
		features           []*geojson.Feature
		detectedGeometries *geos.Geometry
		matchedFeature     *geojson.Feature
	)

	if features, err = baseline.features(); err != nil {
		return nil, err
	}

	if detectedGeometries, err = detected.MultiLineString(); err != nil {
		return nil, err
	}

	// Try to match the geometry for each feature with what we detected
	for _, feature := range features {
		if matchedFeature, err = matchFeature(feature, &detectedGeometries); err != nil {
			return nil, err
		}

		matchedFeatures = append(matchedFeatures, matchedFeature)
	}

	// Construct new features for the geometries that didn't match up
	var newDetection = make(map[string]interface{})
	newDetection[DETECTION] = "New Detection"
	if count, err = detectedGeometries.NGeometry(); err != nil {
		return nil, err
	}
	for inx := 0; inx < count; inx++ {
		var gjGeometry interface{}
		if geometry, err = detectedGeometries.Geometry(inx); err != nil {
			return nil, err
		}
		if gjGeometry, err = fromGeos(geometry); err != nil {
			return nil, err
		}
		matchedFeatures = append(matchedFeatures, geojson.NewFeature(gjGeometry, "", newDetection))
	}

	fc := geojson.NewFeatureCollection(matchedFeatures)
	return fc, nil
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
