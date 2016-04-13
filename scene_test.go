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
	filenameB := "test/baseline.geojson"
	filenameD := "test/detected.geojson"
	baselineScene, err := parseGeoJSONFile(filenameB)
	if err != nil {
		t.Error(err.Error())
	}
	detectedScene, err := parseGeoJSONFile(filenameD)
	if err != nil {
		t.Error(err.Error())
	}
	var envelope *geos.Geometry
	envelope, err = detectedScene.envelope()
	if err != nil {
		t.Error(err.Error())
	}
	log.Printf("Envelope: %v\n", envelope.String())
	err = baselineScene.clip(detectedScene)
	if err != nil {
		t.Error(err.Error())
	}
	envelope, err = baselineScene.envelope()
	if err != nil {
		t.Error(err.Error())
	}
	log.Printf("Envelope: %v\n", envelope.String())
}
