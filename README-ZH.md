中文|[English](README.md) 


# paddleocr

[![Go Reference](https://pkg.go.dev/badge/github.com/doraemonkeys/paddleocr.svg)](https://pkg.go.dev/github.com/doraemonkeys/paddleocr)



Go语言实现的对PaddleOCR-json的简单封装。

## 安装

1. 从[PaddleOCR-json releases](https://github.com/hiroi-sora/PaddleOCR-json/releases)下载程序并解压。
2. 安装paddleocr

   ```go
   go get github.com/doraemonkeys/paddleocr
   ```

## 快速开始
    
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
	fmt.Println(result)
}
```