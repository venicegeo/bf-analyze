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

func main() {
	var (
		args                = os.Args[1:]
		filename, filenameB string
		detectedGeometries  []*geos.Geometry
		geometry            *geos.Geometry
		geojsons            []interface{}
		matchedFeatures     []geojson.Feature
		err                 error
	)
	if len(args) > 1 {
		filename = args[0]
		filenameB = args[1]
	} else {
		filename = "test/detected.geojson"
		filenameB = "test/baseline.geojson"
	}

	// Retrieve the detected features as geometries
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Printf("File read error: %v\n", err)
		os.Exit(1)
	}
	geojsons, err = parseGeoJSONToGeometries(bytes)
	if err != nil {
		log.Printf("GeoJSON parse error: %v\n", err)
		os.Exit(1)
	}

	// Put their geometries into an array for later processing
	for inx := 0; inx < len(geojsons); inx++ {
		geometry, err = toGeos(geojsons[inx])
		if err == nil {
			detectedGeometries = append(detectedGeometries, geometry)
		} else {
			break
		}
	}

	// Join the geometries when possible
	detectedGeometries, err = join(detectedGeometries)
	if err != nil {
		log.Printf("Join error: %v\n", err)
		os.Exit(1)
	}

	// Retrieve the baseline features
	bytes, err = ioutil.ReadFile(filenameB)
	if err != nil {
		log.Printf("File read error: %v\n", err)
		os.Exit(1)
	}
	fcIfc, err := geojson.Parse(bytes)
	if err != nil {
		log.Printf("GeoJSON parse error: %v\n", err)
		os.Exit(1)
	}

	// We are expecting a feature collection
	fc, found := fcIfc.(geojson.FeatureCollection)
	if found {
		features := fc.Features
		// Try to match the geometry for each feature with what we detected
		for inx := 0; inx < len(features); inx++ {
			feature := features[inx]
			matchFeature(&feature, &detectedGeometries)
			matchedFeatures = append(matchedFeatures, feature)
		}

		// Construct new features for the geometries that didn't match up
		var newDetection = make(map[string]interface{})
		newDetection["Detection"] = "New Detection"
		for inx := 0; inx < len(detectedGeometries); inx++ {
			var geometry interface{}
			geometry, err = fromGeos(detectedGeometries[inx])
			if err == nil {
				feature := geojson.Feature{Type: geojson.FEATURE,
					Geometry:   geometry,
					Properties: newDetection}
				matchedFeatures = append(matchedFeatures, feature)
			}
		}
	}

	fc.Features = matchedFeatures
	bytes, err = json.Marshal(fc)
	if err == nil {
		fmt.Printf("%v\n", string(bytes))
	}
}
