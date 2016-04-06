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
func qualitativeReview(detectedGeoJSONs []interface{}, baselineGeoJSONs geojson.FeatureCollection) error {
	var (
		matchedFeatures []geojson.Feature
		geometry        *geos.Geometry
		err             error
		bytes           []byte
		count           int
	)
	// Put the detected geometries into a collection for later processing
	detectedGeometries, err := geos.NewCollection(geos.MULTILINESTRING)
	if err != nil {
		return err
	}

	for inx := 0; inx < len(detectedGeoJSONs); inx++ {
		// Transform the GeoJSON to a GEOS Geometry
		geometry, err = toGeos(detectedGeoJSONs[inx])
		if err == nil {
			// If we get a polygon, we really just want its outer ring for now
			ttype, _ := geometry.Type()
			if ttype == geos.POLYGON {
				geometry, err = geometry.Shell()
			}
			if err == nil {
				detectedGeometries, err = detectedGeometries.Union(geometry)
			}
		}
		if err != nil {
			return err
		}
	}

	// Join the geometries when possible
	detectedGeometries, err = detectedGeometries.LineMerge()
	if err != nil {
		return err
	}

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

func main() {
	var (
		args                = os.Args[1:]
		filename, filenameB string
		detectedGeometries  []interface{}
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
	// Pluck the geometries into an array
	detectedGeometries = geojson.ToGeometryArray(detectedGeoJSON)

	// Retrieve the baseline features as GeoJSON
	baselineGeoJSON, err = parseGeoJSONFile(filenameB)
	if err != nil {
		log.Printf("File read error: %v\n", err)
		os.Exit(1)
	}
	fc, found := baselineGeoJSON.(geojson.FeatureCollection)
	if found {
		qualitativeReview(detectedGeometries, fc)
	} else {
		log.Print("Baseline input must be a GeoJSON FeatureCollection.")
		os.Exit(1)
	}
}
