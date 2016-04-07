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
	"io/ioutil"
	"log"
	"os"

	"github.com/paulsmith/gogeos/geos"
	"github.com/venicegeo/geojson-go/geojson"
)

func parseGeoJSONFile(filename string) (interface{}, error) {
	var result interface{}
	bytes, err := ioutil.ReadFile(filename)
	if err == nil {
		result, err = geojson.Parse(bytes)
	}
	return result, err
}
func qualitativeReview(detectedGeometries *geos.Geometry, baselineGeoJSONs geojson.FeatureCollection) error {
	var (
		matchedFeatures []geojson.Feature
		geometry        *geos.Geometry
		err             error
		bytes           []byte
		count           int
	)

	features := baselineGeoJSONs.Features

	// Try to match the geometry for each feature with what we detected
	for inx := 0; inx < len(features); inx++ {
		feature := features[inx]
		matchFeature(&feature, detectedGeometries)
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
			break
		}
		gjGeometry, err = fromGeos(geometry)
		if err == nil {
			feature := geojson.Feature{Type: geojson.FEATURE,
				Geometry:   gjGeometry,
				Properties: newDetection}
			matchedFeatures = append(matchedFeatures, feature)
		} else {
			break
		}
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
	envelopeArea, totalArea float64
	positive                bool
}

func quantitativeReview(detectedGeometries *geos.Geometry) error {
	var (
		err             error
		polygon         *geos.Geometry
		mpolygon        *geos.Geometry
		envelope        *geos.Geometry
		count           int
		currentMetadata polygonMetadata
	)

	mpolygon, err = mlsToMultiPolygon(detectedGeometries)
	if err != nil {
		return err
	}
	count, err = mpolygon.NGeometry()
	for inx := 0; inx < count; inx++ {
		polygon, err = mpolygon.Geometry(inx)
		if err != nil {
			break
		}
		envelope, err = polygon.Envelope()
		if err != nil {
			break
		}
		currentMetadata.totalArea, err = polygon.Area()
		if err != nil {
			break
		}
		currentMetadata.envelopeArea, err = envelope.Area()
		if err != nil {
			break
		}
		// for jnx := 0; jnx < count; jnx++ {
		//   polygon, err = mpolygon.Geometry(inx)
		//
		// }
		fmt.Printf("%#v\n", currentMetadata)
	}
	return err
	// extents, err := detectedGeometries.Envelope()
	// if err != nil {
	// 	return err
	// }
	// polygon, err := geos.NewPolygon(extents, detectedGeometries...)
	// baselineGeometry, err = mlsFromGeoJSON(baselineGeoJSON)
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("%v\n", baselineGeometry.String())
	// baselineGeometry, err = baselineGeometry.Intersection(extents)
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("%v\n", baselineGeometry.String())
	// outerRing, err = extents.Shell()
	// if err != nil {
	// 	return err
	// }
	// baselineGeometry, err = baselineGeometry.Union(outerRing)
	// if err != nil {
	// 	return err
	// }
}

func mlsFromGeoJSON(input interface{}) (*geos.Geometry, error) {
	var (
		geometry *geos.Geometry
		err      error
	)

	result, err := geos.NewCollection(geos.MULTILINESTRING)

	// Pluck the geometries into an array
	detectedGJGeometries := geojson.ToGeometryArray(input)

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
	return result, err
}

func main() {
	var (
		args                = os.Args[1:]
		filename, filenameB string
		detectedGeometries  *geos.Geometry
		baselineGeometries  *geos.Geometry
		detectedEnvelope    *geos.Geometry
		detectedGeoJSON     interface{}
		baselineGeoJSON     interface{}
		err                 error
	)
	if len(args) > 1 {
		filename = args[0]
		filenameB = args[1]
	} else {
		filename = "test/detected.geojson"
		filenameB = "test/baseline.geojson"
	}

	// Retrieve the detected features as GeoJSON
	detectedGeoJSON, err = parseGeoJSONFile(filename)
	if err != nil {
		log.Printf("File read error: %v\n", err)
		os.Exit(1)
	}
	detectedGeometries, err = mlsFromGeoJSON(detectedGeoJSON)
	if err != nil {
		log.Printf("Could not prepare detected geometries for analysis: %v\n", err)
		os.Exit(1)
	}

	// Retrieve the baseline features as GeoJSON
	baselineGeoJSON, err = parseGeoJSONFile(filenameB)
	if err != nil {
		log.Printf("File read error: %v\n", err)
		os.Exit(1)
	}
	// fc, found := baselineGeoJSON.(geojson.FeatureCollection)
	// if found {
	// qualitativeReview(detectedGeometries, fc)
	err = quantitativeReview(detectedGeometries)
	if err != nil {
		log.Printf("Quantitative review failed: %v\n", err)
		os.Exit(1)
	}
	baselineGeometries, err = mlsFromGeoJSON(baselineGeoJSON)
	if err != nil {
		log.Printf("Could not prepare baseline geometries for analysis: %v\n", err)
		os.Exit(1)
	}
	detectedEnvelope, err = detectedGeometries.Envelope()
	if err != nil {
		log.Printf("Come on. Detected geometries have no envelope?!? %v\n", err)
		os.Exit(1)
	}
	baselineGeometries, err = detectedEnvelope.Intersection(baselineGeometries)
	if err != nil {
		log.Printf("Failed to calculate intersection of baseline and detected features. %v\n", err)
		os.Exit(1)
	}
	err = quantitativeReview(baselineGeometries)
	if err != nil {
		log.Printf("Quantitative review failed: %v\n", err)
		os.Exit(1)
	}
	// } else {
	// 	log.Print("Baseline input must be a GeoJSON FeatureCollection.")
	// 	os.Exit(1)
	// }
}
