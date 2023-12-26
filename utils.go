package paddleocr

import "os"

// 文件是否存在
func fileIsExist(path string) bool {
	f, err := os.Stat(path)
	if err != nil {
		return os.IsExist(err)
	}
	if f.IsDir() {
		return false
	}
	return true
}
