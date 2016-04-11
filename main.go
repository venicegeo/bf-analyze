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
		matchFeature(&feature, &detectedGeometries)
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
	boundaryArea, totalArea float64
	terminal                bool
	link                    *polygonMetadata
	index                   int
}

func quantitativeReview(detectedGeometries *geos.Geometry) error {
	var (
		err                        error
		polygon                    *geos.Geometry
		polygon2                   *geos.Geometry
		mpolygon                   *geos.Geometry
		boundary                   *geos.Geometry
		count                      int
		touches                    bool
		positiveArea, negativeArea float64
	)

	mpolygon, err = mlsToMPoly(detectedGeometries)
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

	// Retrieve the detected features as a GeoJSON MultiLineString
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

	// Retrieve the baseline features as a GeoJSON MultiLineString
	baselineGeoJSON, err = parseGeoJSONFile(filenameB)
	if err != nil {
		log.Printf("File read error: %v\n", err)
		os.Exit(1)
	}
	baselineGeometries, err = mlsFromGeoJSON(baselineGeoJSON)
	if err != nil {
		log.Printf("Could not prepare baseline geometries for analysis: %v\n", err)
		os.Exit(1)
	}

	// Clip the baseline features with the envelope of the detected features
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

	// Qualitative Review: What features match, are new, or are missing
	fc, found := baselineGeoJSON.(geojson.FeatureCollection)
	if found {
		qualitativeReview(detectedGeometries, fc)
	} else {
		log.Print("Baseline input must be a GeoJSON FeatureCollection.")
		os.Exit(1)
	}

	// Quantitative Review: what is the land/water area for the two
	err = quantitativeReview(detectedGeometries)
	if err != nil {
		log.Printf("Quantitative review failed: %v\n", err)
		os.Exit(1)
	}

	err = quantitativeReview(baselineGeometries)
	if err != nil {
		log.Printf("Quantitative review failed: %v\n", err)
		os.Exit(1)
	}
}
