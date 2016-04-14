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
	"testing"

	"github.com/paulsmith/gogeos/geos"
)

// TestScene Unit test for this object
func TestScene(t *testing.T) {
	var (
		envelope      *geos.Geometry
		baselineScene Scene
		detectedScene Scene
		err           error
	)
	filenameB := "test/baseline.geojson"
	filenameD := "test/detected.geojson"
	if baselineScene, err = parseGeoJSONFile(filenameB); err != nil {
		t.Error(err.Error())
	}
	if detectedScene, err = parseGeoJSONFile(filenameD); err != nil {
		t.Error(err.Error())
	}
	if envelope, err = detectedScene.envelope(); err != nil {
		t.Error(err.Error())
	}
	log.Printf("Envelope: %v\n", envelope.String())
	if err = baselineScene.clip(detectedScene); err != nil {
		t.Error(err.Error())
	}
	if envelope, err = baselineScene.envelope(); err != nil {
		t.Error(err.Error())
	}
	log.Printf("Envelope: %v\n", envelope.String())
}
