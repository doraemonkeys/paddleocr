English|[中文](/README-ZH.md)  



# paddleocr

[![Go Reference](https://pkg.go.dev/badge/github.com/doraemonkeys/paddleocr.svg)](https://pkg.go.dev/github.com/doraemonkeys/paddleocr) [![Go Report Card](https://goreportcard.com/badge/github.com/doraemonkeys/paddleocr)](https://goreportcard.com/report/github.com/doraemonkeys/paddleocr)


A simple wrapper for hiroi-sora/PaddleOCR-json implemented in Go language.


## Installation

1. Download the program from [PaddleOCR-json releases](https://github.com/hiroi-sora/PaddleOCR-json/releases) and decompress it.

2. install paddleocr

   ```go
   go get github.com/doraemonkeys/paddleocr
   ```

## Quick Start

```go
package main

import (
	"fmt"

	"github.com/doraemonkeys/paddleocr"
)

func main() {
	p, err := paddleocr.NewPpocr("path/to/PaddleOCR-json.exe",
		paddleocr.OcrArgs{})
	if err != nil {
		panic(err)
	}
	defer p.Close()
	result, err := p.OcrFileAndParse(`path/to/image.png`)
	if err != nil {
		panic(err)
	}
	if result.Code != paddleocr.CodeSuccess {
		fmt.Println("orc failed:", result.Msg)
		return
	}
	fmt.Println(result.Data)
}
```

