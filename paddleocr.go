package paddleocr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

type OcrArgs struct {
	// 启用cls方向分类，识别方向不是正朝上的图片。默认为false。
	Cls *bool `paddleocr:"cls"`
	// 启用CPU推理加速，关掉可以减少内存占用，但会降低速度。默认为true。
	EnableMkldnn *bool `paddleocr:"enable_mkldnn"`
	// 若图片长边长度大于该值，会被缩小到该值，以提高速度。默认为960。
	// 如果对大图/长图的识别率低，可增大 limit_side_len 的值。
	// 建议为 32 & 48 的公倍数，如 960, 2880, 4320
	LimitSideLen *int32 `paddleocr:"limit_side_len"`
	// 启用方向分类，必须与cls值相同。 默认为false。
	UseAngleCls *bool `paddleocr:"use_angle_cls"`
	// 指定不同语言的配置文件路径，识别多国语言。
	// models 目录中，每一个 config_xxx.txt 是一组语言配置文件（如英文是congfig_en.txt）。
	// 只需将这个文件的路径传入 config_path 参数，即可切换为对应的语言。
	//
	// 例如：
	//   paddleocr.OcrArgs{ ConfigPath: paddleocr.ConfigChinese }
	ConfigPath string `paddleocr:"config_path"`
}

const paddleocrTag = "paddleocr"

const (
	ConfigChinese    = "models/config_chinese.txt"
	ConfigChineseCht = "models/config_chinese_cht.txt"
	ConfigCyrillic   = "models/config_cyrillic.txt"
	ConfigEn         = "models/config_en.txt"
	ConfigFrenchV2   = "models/config_french_v2.txt"
	ConfigGermanV2   = "models/config_german_v2.txt"
	ConfigJapan      = "models/config_japan.txt"
	ConfigKorean     = "models/config_korean.txt"
)

const clipboardImagePath = `clipboard`

func (o OcrArgs) CmdString() string {
	var s string
	v := reflect.ValueOf(o)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).IsZero() {
			continue
		}
		f := v.Type().Field(i)
		if f.Tag.Get(paddleocrTag) == "" {
			continue
		}
		// value := v.Field(i).Elem().Interface()
		value := v.Field(i).Interface()

		switch valueType := value.(type) {
		case *bool:
			if *valueType {
				s += fmt.Sprintf("%s=1 ", f.Tag.Get(paddleocrTag))
			} else {
				s += fmt.Sprintf("%s=0 ", f.Tag.Get(paddleocrTag))
			}
		default:
			if v.Field(i).Kind() == reflect.Ptr {
				s += fmt.Sprintf("%s=%v ", f.Tag.Get(paddleocrTag), v.Field(i).Elem().Interface())
			} else {
				s += fmt.Sprintf("%s=%v ", f.Tag.Get(paddleocrTag), value)
			}
		}
	}
	s = strings.TrimSpace(s)
	return s
}

// OcrFile processes the OCR for a given image file path using the specified OCR arguments.
// It returns the raw OCR result as bytes and any error encountered.
func OcrFile(exePath, imagePath string, argsCnf OcrArgs) ([]byte, error) {
	p, err := NewPpocr(exePath, argsCnf)
	if err != nil {
		return nil, err
	}
	defer p.Close()
	b, err := p.OcrFile(imagePath)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func OcrFileAndParse(exePath, imagePath string, argsCnf OcrArgs) (Result, error) {
	data, err := OcrFile(exePath, imagePath, argsCnf)
	if err != nil {
		return Result{}, err
	}
	return ParseResult(data)
}

type Ppocr struct {
	exePath         string
	args            OcrArgs
	ppLock          *sync.Mutex
	restartExitChan chan struct{}
	internalErr     error

	cmdStdout io.ReadCloser
	cmdStdin  io.WriteCloser
	cmd       *exec.Cmd
	// 无缓冲同步信号通道，close()中接收，Run()中发送。
	// Run()退出必须有对应close方法的调用
	runGoroutineExitedChan chan struct{}
	// startTime time.Time
}

// NewPpocr creates a new instance of the Ppocr struct with the provided executable path
// and OCR arguments.
// It initializes the OCR process and returns a pointer to the Ppocr instance
// and any error encountered.
//
// It is the caller's responsibility to close the Ppocr instance when finished.
func NewPpocr(exePath string, args OcrArgs) (*Ppocr, error) {
	if !fileIsExist(exePath) {
		return nil, fmt.Errorf("executable file %s not found", exePath)
	}
	p := &Ppocr{
		exePath:                exePath,
		args:                   args,
		ppLock:                 new(sync.Mutex),
		restartExitChan:        make(chan struct{}),
		runGoroutineExitedChan: make(chan struct{}),
	}

	p.ppLock.Lock()
	defer p.ppLock.Unlock()
	err := p.initPpocr(exePath, args)
	if err == nil {
		go p.restartTimer()
	} else {
		p.close()
	}
	return p, err
}

// 加锁调用，发生错误需要close
func (p *Ppocr) initPpocr(exePath string, args OcrArgs) error {
	var cmdSlash string
	if runtime.GOOS == "windows" {
		cmdSlash = "\\"
	} else {
		cmdSlash = "/"
	}
	p.cmd = exec.Command("."+cmdSlash+filepath.Base(exePath), strings.Fields(args.CmdString())...)
	cmdDir := filepath.Dir(exePath)
	if cmdDir == "." {
		cmdDir = ""
	}
	p.cmd.Dir = cmdDir
	wc, err := p.cmd.StdinPipe()
	if err != nil {
		return err
	}
	rc, err := p.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	p.cmdStdin = wc
	p.cmdStdout = rc

	var stderrBuffer bytes.Buffer
	p.cmd.Stderr = &stderrBuffer

	err = p.cmd.Start()
	if err != nil {
		return fmt.Errorf("OCR process start failed: %v", err)
	}

	go func() {
		p.internalErr = nil
		err := p.cmd.Wait()
		// fmt.Println("Run() OCR process exited, error:", err)
		if err != nil {
			p.internalErr = err
		}
		p.runGoroutineExitedChan <- struct{}{}
	}()

	buf := make([]byte, 4096)
	start := 0
	for {
		n, err := rc.Read(buf[start:])
		if err != nil {
			if p.internalErr != nil {
				return fmt.Errorf("OCR init failed: %v,run error: %v", err, p.internalErr)
			}
			return fmt.Errorf("OCR init failed, error: %v, output: %s %s", err, buf[:start], stderrBuffer.String())
		}
		start += n
		if start >= len(buf) {
			return fmt.Errorf("OCR init failed: output too long")
		}
		if bytes.Contains(buf[:start], []byte("OCR init completed.")) {
			break
		}
	}
	return p.internalErr
}

// Close cleanly shuts down the OCR process associated with the Ppocr instance.
// It releases any resources and terminates the OCR process.
//
// Warning: This method should only be called once.
func (p *Ppocr) Close() error {
	p.ppLock.Lock()
	defer p.ppLock.Unlock()
	// close(p.restartExitChan) // 只能关闭一次
	select {
	case <-p.restartExitChan:
		return fmt.Errorf("OCR process has been closed")
	default:
		close(p.restartExitChan)
	}
	p.internalErr = fmt.Errorf("OCR process has been closed")
	return p.close()
}

func (p *Ppocr) close() (err error) {
	select {
	case <-p.runGoroutineExitedChan:
		return nil
	default:
	}
	defer func() {
		// 可能的情况：Run刚退出，p.exited还没设置为true
		if r := recover(); r != nil {
			err = fmt.Errorf("close panic: %v", r)
		}
		// fmt.Println("wait OCR runGoroutineExitedChan")
		<-p.runGoroutineExitedChan
		// fmt.Println("wait OCR runGoroutineExitedChan done")
	}()
	if p.cmd == nil {
		return nil
	}
	if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
		return nil
	}
	if err := p.cmdStdin.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close cmdIn error: %v\n", err)
	}
	if err := p.cmd.Process.Kill(); err != nil {
		return err
	}
	// fmt.Println("kill OCR process success")
	return nil
}

// 定时重启进程减少内存占用(ocr程序有内存泄漏)
func (p *Ppocr) restartTimer() {
	// ticker := time.NewTicker(10 * time.Second)
	ticker := time.NewTicker(20 * time.Minute)
	for {
		select {
		case <-ticker.C:
			// fmt.Println("restart OCR process")
			p.ppLock.Lock()
			_ = p.close()
			p.internalErr = p.initPpocr(p.exePath, p.args)
			p.ppLock.Unlock()
			// fmt.Println("restart OCR process done")
		case <-p.restartExitChan:
			// fmt.Println("exit OCR process")
			return
		}
	}
}

type imageData struct {
	Path       string `json:"image_path,omitempty"`
	ContentB64 []byte `json:"image_base64,omitempty"`
}

// OcrFile sends an image file path to the OCR process and retrieves the OCR result.
// It returns the OCR result as bytes and any error encountered.
func (p *Ppocr) OcrFile(imagePath string) ([]byte, error) {
	var data = imageData{Path: imagePath}
	dataJson, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	p.ppLock.Lock()
	defer p.ppLock.Unlock()
	if p.internalErr != nil {
		return nil, p.internalErr
	}
	return p.ocr(dataJson)
}

func (p *Ppocr) ocr(dataJson []byte) ([]byte, error) {
	_, err := p.cmdStdin.Write(dataJson)
	if err != nil {
		return nil, err
	}
	_, err = p.cmdStdin.Write([]byte("\n"))
	if err != nil {
		return nil, err
	}
	content := make([]byte, 1024*10)
	start := 0
	for {
		n, err := p.cmdStdout.Read(content[start:])
		if err != nil {
			return nil, err
		}
		start += n
		if start >= len(content) {
			content = append(content, make([]byte, 1024*10)...)
		}
		if content[start-1] == '\n' {
			break
		}
	}
	return content[:start], nil
}

// Ocr processes the OCR for a given image represented as a byte slice.
// It returns the OCR result as bytes and any error encountered.
func (p *Ppocr) Ocr(image []byte) ([]byte, error) {
	if p.internalErr != nil {
		return nil, p.internalErr
	}
	var data = imageData{ContentB64: image}
	dataJson, err := json.Marshal(data) //auto base64
	if err != nil {
		return nil, err
	}

	p.ppLock.Lock()
	defer p.ppLock.Unlock()
	return p.ocr(dataJson)
}

type Data struct {
	Rect  [][]int `json:"box"`
	Score float32 `json:"score"`
	Text  string  `json:"text"`
}

type Result struct {
	Code int
	Msg  string
	Data []Data
}

const (
	// CodeSuccess indicates that the OCR process was successful.
	CodeSuccess = 100
	// CodeNoText indicates that no text was recognized.
	CodeNoText = 101
)

// ParseResult parses the raw OCR result bytes into a slice of Result structs.
// It returns the parsed results and any error encountered during parsing.
func ParseResult(rawData []byte) (Result, error) {
	var resp map[string]any
	err := json.Unmarshal(rawData, &resp)
	if err != nil {
		return Result{}, err
	}
	var result = Result{}
	var resData = make([]Data, 0)
	if resp["code"] == nil {
		return Result{}, fmt.Errorf("no code in response")
	}
	if resp["code"].(float64) != 100 {
		result.Code = int(resp["code"].(float64))
		result.Msg = fmt.Sprintf("%v", resp["data"])
		return result, nil
	}
	if resp["data"] == nil {
		return Result{}, fmt.Errorf("no data in response")
	}
	dataSlice, ok := resp["data"]
	if !ok {
		return result, fmt.Errorf("data is not array")
	}
	result.Code = CodeSuccess
	result.Msg = "parse success"

	var data []any
	data, ok = dataSlice.([]any)
	if !ok {
		return result, fmt.Errorf("data is not array")
	}
	for _, v := range data {
		str, err := json.Marshal(v)
		if err != nil {
			return result, err
		}
		var r Data
		err = json.Unmarshal(str, &r)
		if err != nil {
			return result, err
		}
		resData = append(resData, r)
	}
	result.Data = resData
	return result, nil
}

// OcrFileAndParse processes the OCR for a given image file path and parses the result.
// It returns the parsed OCR results as a slice of Result structs and any error encountered.
func (p *Ppocr) OcrFileAndParse(imagePath string) (Result, error) {
	b, err := p.OcrFile(imagePath)
	if err != nil {
		return Result{}, err
	}
	return ParseResult(b)
}

// OcrAndParse processes and parses the OCR for a given image represented as a byte slice.
// It returns the parsed OCR results as a slice of Result structs and any error encountered.
func (p *Ppocr) OcrAndParse(image []byte) (Result, error) {
	b, err := p.Ocr(image)
	if err != nil {
		return Result{}, err
	}
	return ParseResult(b)
}

// OcrClipboard processes the OCR for an image stored in the clipboard.
// It returns the raw OCR result as bytes and any error encountered.
func (p *Ppocr) OcrClipboard() ([]byte, error) {
	return p.OcrFile(clipboardImagePath)
}

// OcrClipboardAndParse processes the OCR for an image stored in the clipboard and parses the result.
// It returns the parsed OCR results as a slice of Result structs and any error encountered.
func (p *Ppocr) OcrClipboardAndParse() (Result, error) {
	return p.OcrFileAndParse(clipboardImagePath)
}
