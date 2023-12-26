package paddleocr

import (
	"testing"
)

func TestOcrArgs_CmdString(t *testing.T) {
	truePtr := new(bool)
	*truePtr = true
	intPtr := new(int32)
	*intPtr = 960

	tests := []struct {
		name string
		o    OcrArgs
		want string
	}{
		{"1", OcrArgs{Cls: nil, EnableMkldnn: nil, LimitSideLen: nil, UseAngleCls: nil},
			""},
		{"2", OcrArgs{Cls: new(bool), EnableMkldnn: nil, LimitSideLen: nil, UseAngleCls: nil},
			"cls=0"},
		{"3", OcrArgs{Cls: truePtr, EnableMkldnn: nil, LimitSideLen: nil, UseAngleCls: nil},
			"cls=1"},
		{"4", OcrArgs{Cls: truePtr, EnableMkldnn: truePtr, LimitSideLen: nil, UseAngleCls: nil},
			"cls=1 enable_mkldnn=1"},
		{"5", OcrArgs{Cls: truePtr, EnableMkldnn: truePtr, LimitSideLen: new(int32), UseAngleCls: nil},
			"cls=1 enable_mkldnn=1 limit_side_len=0"},
		{"6", OcrArgs{Cls: truePtr, EnableMkldnn: truePtr, LimitSideLen: intPtr, UseAngleCls: nil},
			"cls=1 enable_mkldnn=1 limit_side_len=960"},
		{"7", OcrArgs{Cls: truePtr, EnableMkldnn: truePtr, LimitSideLen: intPtr, UseAngleCls: truePtr},
			"cls=1 enable_mkldnn=1 limit_side_len=960 use_angle_cls=1"},
		{"8", OcrArgs{Cls: truePtr, EnableMkldnn: truePtr, LimitSideLen: intPtr, UseAngleCls: truePtr, ConfigPath: ConfigChinese},
			"cls=1 enable_mkldnn=1 limit_side_len=960 use_angle_cls=1 config_path=models/config_chinese.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.o.CmdString(); got != tt.want {
				t.Errorf("OcrArgs.CmdString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewPpocr(t *testing.T) {
	type args struct {
		exePath string
		args    OcrArgs
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"1", args{"",
			OcrArgs{}}, true},
		{"2", args{`D:\用户\computer\Downloads\PaddleOCR-json_v.1.3.1\PaddleOCR-json_v.1.3.1\PaddleOCR-json.exe`,
			OcrArgs{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPpocr(tt.args.exePath, tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPpocr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
