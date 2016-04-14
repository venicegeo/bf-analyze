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
	"os"

	"github.com/paulsmith/gogeos/geos"
)

func main() {
	var (
		args                = os.Args[1:]
		filename, filenameB string
		detectedEnvelope    *geos.Geometry
		err                 error
		detected, baseline  Scene
	)
	if len(args) > 1 {
		filename = args[0]
		filenameB = args[1]
	} else {
		filename = "test/detected.geojson"
		filenameB = "test/baseline.geojson"
	}

	// Retrieve the detected features as a GeoJSON MultiLineString
	if detected, err = parseGeoJSONFile(filename); err != nil {
		log.Printf("File read error: %v\n", err)
		os.Exit(1)
	}

	// Retrieve the baseline features as a GeoJSON MultiLineString
	if baseline, err = parseGeoJSONFile(filenameB); err != nil {
		log.Printf("File read error: %v\n", err)
		os.Exit(1)
	}

	if err = baseline.clip(detected); err != nil {
		log.Printf("Could not clip baseline: %v\n", err)
		os.Exit(1)
	}

	// Qualitative Review: What features match, are new, or are missing
	if err = qualitativeReview(detected, baseline); err != nil {
		log.Printf("Qualitative Review failed: %v\n", err)
		os.Exit(1)
	}

	// Quantitative Review: what is the land/water area for the two
	if detectedEnvelope, err = detected.envelope(); err != nil {
		log.Printf("Could not retrieve envelope: %v\n", err)
		os.Exit(1)
	}
	if err = quantitativeReview(baseline, detectedEnvelope); err != nil {
		log.Printf("Quantitative review failed: %v\n", err)
		os.Exit(1)
	}

	if err = quantitativeReview(detected, detectedEnvelope); err != nil {
		log.Printf("Quantitative review failed: %v\n", err)
		os.Exit(1)
	}
}
