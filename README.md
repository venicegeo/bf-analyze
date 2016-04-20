# bf-analyze

## Overview
A toolkit for analyzing the results of BeachFront shoreline detection.

### Dependencies
`bf-analyze` is written in Go. Use `go get` to get its dependencies.

#### GEOS
It depends on go-geos which requires GEOS, a C/C++ library.
- https://github.com/paulsmith/gogeos#installation

#### bf-line-analyzer
The metrics (see below) use a GEOS-based C++ application called `bf-line-analyzer`.
- https://github.com/venicegeo/bf-line-analyzer
- mkdir bld
- cd bld
- make
- Set an environment variable `BF_LINE_ANALYZER_DIR` to be the directory of the repository.

### Building
1. `go build`

### What it Does
`./bf-analyze [detected] [baseline]` writes output of the detection to standard output.

#### Qualitative Analysis
The output is GeoJSON with a `Detection` property on each feature.

* `Detected` means the detected shoreline is in the baseline. 
In this case the geometry will be a GeometryCollection consisting of the detected geometry followed by the baseline geometry.
* `Not Detected` means a feature in the baseline was not detected.
* `New Detection` means a feature in the detected file does not have a corresponding entry in the baseline.

##### Metrics
When `Detected`, we run some simple metrics on the baseline and detected. 
These are added as properties to the GeoJSON feature.

#### Quantitative Analysis
The quantitative analysis determines the amount of positive/negative space in a scene.
It constructs a MultiPolygon from the linework and then measures the area of each component polygon.
Area is measured twice - boundary area and total area (inner rings are not counted as part of total area.
From there we can output the sum of the positive and negative space in the scene. 
For now this output is logged; a better way to do this is TBD.

  
