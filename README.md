# bf-analyze

## Overview
A toolkit for analyzing the results of BeachFront shoreline detection.

### Building
`bf-analyze` is written in Go and depends on go-geos which requires GEOS, a C/C++ library.

1. https://github.com/paulsmith/gogeos#installation
1. `go build`

### What it Does
`./bf-analyze [detected] [baseline] writes output of the detection to standard output.
The output is GeoJSON with a `Detection` property on each feature.

* `Detected` means the detected shoreline is in the baseline. 
In this case the geometry will be a GeometryCollection consisting of the detected geometry followed by the baseline geometry.
* `Not Detected` means a feature in the baseline was not detected.
* `New Detection` means a feature in the detected file does not have a corresponding entry in the baseline.


