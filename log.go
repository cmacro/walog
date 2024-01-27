// log
//
// strange situation?
// parameter inside. If this waLog is placed in pkg dir, the log_dir parameter cannot be used, an exception occurred.
//
// use the standard log mode directly, log cutting is handled by the system. reduce the memory usage.
//

package walog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

var (
	calldepth = 3

	ColorLog = false

	infoLog *log.Logger
	errLog  *log.Logger
)

// Logger is a simple logger interface that can have subloggers for specific areas.
type Logger interface {
	Warnf(msg string, args ...interface{})
	Errorf(msg string, args ...interface{})
	Infof(msg string, args ...interface{})
	Debugf(msg string, args ...interface{})
	Fatalf(msg string, args ...interface{})
	Warn(msgs ...interface{})
	Error(msgs ...interface{})
	Info(msgs ...interface{})
	Debug(msgs ...interface{})
	DebugJsonf(_ string, _ ...interface{})
	Sync()

	Write(p []byte) (n int, err error)

	Sub(module string) Logger
}

type noopLogger struct{}

func (n *noopLogger) Errorf(_ string, _ ...interface{})     {}
func (n *noopLogger) Warnf(_ string, _ ...interface{})      {}
func (n *noopLogger) Infof(_ string, _ ...interface{})      {}
func (n *noopLogger) Debugf(_ string, _ ...interface{})     {}
func (n *noopLogger) Fatalf(_ string, _ ...interface{})     { os.Exit(1) }
func (n *noopLogger) Warn(_ ...interface{})                 {}
func (n *noopLogger) Error(_ ...interface{})                {}
func (n *noopLogger) Info(_ ...interface{})                 {}
func (n *noopLogger) Debug(_ ...interface{})                {}
func (n *noopLogger) DebugJsonf(_ string, _ ...interface{}) {}
func (s *noopLogger) Write(p []byte) (n int, err error)     { return 0, nil }
func (s *noopLogger) Sync()                                 {}

func (n *noopLogger) Sub(_ string) Logger { return n }

// Noop is a no-op Logger implementation that silently drops everything.
var (
	Noop Logger = &noopLogger{}

	bufPool = sync.Pool{
		New: func() any {
			// The Pool's New function should generally only return pointer
			// types, since a pointer can be put into the return interface
			// value without an allocation:
			return new(bytes.Buffer)
		},
	}
)

type stdoutLogger struct {
	mod   string
	color bool
	min   int
}

var colors = map[string]string{
	"INFO":  "\033[36m",
	"WARN":  "\033[33m",
	"ERROR": "\033[31m",
}

var levelToInt = map[string]int{
	"":      -1,
	"DEBUG": 0,
	"INFO":  1,
	"WARN":  2,
	"ERROR": 3,
}

var LevelToSeverity = map[string]int{
	"":      0,
	"DEBUG": 0,
	"INFO":  0,
	"WARN":  1,
	"ERROR": 2,
}

func (s *stdoutLogger) outputf(level, msg string, args ...interface{}) {
	if levelToInt[level] < s.min {
		return
	}
	var colorStart, colorReset string
	if s.color {
		colorStart = colors[level]
		colorReset = "\033[0m"
	}
	outmsg := msg
	if len(args) > 0 {
		outmsg = fmt.Sprintf(msg, args...)
	}
	// outmsg = fmt.Sprint(colorStart, "[", s.mod, " ", level, "]", outmsg, colorReset)
	bf := bufPool.Get().(*bytes.Buffer)
	bf.Reset()
	if colorStart != "" {
		bf.WriteString(colorStart)
	}

	bf.WriteByte('[')
	bf.WriteString(s.mod)
	bf.WriteByte(' ')
	bf.WriteString(level)
	bf.WriteByte(']')
	bf.WriteString(outmsg)
	if colorReset != "" {
		bf.WriteString(colorReset)
	}
	_ = log.Output(calldepth, bf.String())
	bufPool.Put(bf)

}

func (s *stdoutLogger) Write(p []byte) (n int, err error) {
	// outmsg := msg
	// if len(args) > 0 {
	// 	outmsg = fmt.Sprintf(msg, args...)
	// }
	outmsg := fmt.Sprint("[", s.mod, "] ", string(p))
	_ = infoLog.Output(calldepth-1, outmsg)
	return len(p), nil
}

func (s *stdoutLogger) output(level string, msgs ...interface{}) {
	if levelToInt[level] < s.min {
		return
	}

	bf := bufPool.Get().(*bytes.Buffer)
	bf.Reset()

	// var args []interface{}
	if !s.color {
		bf.WriteByte('[')
		bf.WriteString(s.mod)
		bf.WriteByte(' ')
		bf.WriteString(level)
		bf.WriteByte(']')
		writeToBf(bf, msgs...)

	} else {
		bf.WriteString(colors[level])
		bf.WriteByte('[')
		bf.WriteString(s.mod)
		bf.WriteByte(' ')
		bf.WriteString(level)
		bf.WriteByte(']')
		writeToBf(bf, msgs...)
		bf.WriteString("\033[0m")
	}
	data := bf.String()
	bufPool.Put(bf)

	_ = infoLog.Output(calldepth, data)
	if level == "ERROR" {
		_ = errLog.Output(calldepth, data)
	}
}

func (s *stdoutLogger) Fatalf(msg string, args ...interface{}) {
	s.outputf("ERROR", msg, args...)
	os.Exit(1)
}
func (s *stdoutLogger) Errorf(msg string, args ...interface{}) { s.outputf("ERROR", msg, args...) }
func (s *stdoutLogger) Warnf(msg string, args ...interface{})  { s.outputf("WARN", msg, args...) }
func (s *stdoutLogger) Infof(msg string, args ...interface{})  { s.outputf("INFO", msg, args...) }
func (s *stdoutLogger) Debugf(msg string, args ...interface{}) { s.outputf("DEBUG", msg, args...) }
func (s *stdoutLogger) Warn(msgs ...interface{})               { s.output("WARN", msgs...) }
func (s *stdoutLogger) Error(msgs ...interface{})              { s.output("ERROR", msgs...) }
func (s *stdoutLogger) Info(msgs ...interface{})               { s.output("INFO", msgs...) }
func (s *stdoutLogger) Debug(msgs ...interface{})              { s.output("DEBUG", msgs...) }
func (s *stdoutLogger) Sync()                                  {}
func (s *stdoutLogger) DebugJsonf(msg string, args ...interface{}) {
	//s.outputf("DEBUG", msg, args...)
	if levelToInt["DEBUG"] < s.min {
		return
	}
	var d []byte
	if len(args) == 1 {
		d, _ = json.Marshal(args[0])
	}
	s.output("DEBUG", msg, string(d))
}

func (s *stdoutLogger) Sub(mod string) Logger {
	return &stdoutLogger{mod: fmt.Sprintf("%s/%s", s.mod, mod), color: s.color, min: s.min}
}

// Stdout is a simple Logger implementation that outputs to stdout. The module name given is included in log lines.
//
// minLevel specifies the minimum log level to output. An empty string will output all logs.
//
// If color is true, then info, warn and error logs will be colored cyan, yellow and red respectively using ANSI color escape codes.
func Stdout(module string, minLevel string, color bool) Logger {
	if infoLog == nil {
		infoLog = log.New(os.Stdout, "", log.LstdFlags)
		errLog = log.New(os.Stderr, "", log.LstdFlags+log.Lshortfile)
	}
	return &stdoutLogger{mod: module, color: color, min: levelToInt[strings.ToUpper(minLevel)]}
}

func LogSubName(_ string) {}

func writeToBf(p *bytes.Buffer, args ...any) {

	for argNum, arg := range args {
		if argNum > 0 {
			p.WriteByte(' ')
		}
		p.WriteString(v2s(arg))
	}
}

func v2s(value interface{}) (data string) {
	if value == nil {
		return
	}

	switch value := value.(type) {
	case error:
		data = value.Error()
	case float64:
		data = strconv.FormatFloat(value, 'f', -1, 64)
	case float32:
		data = strconv.FormatFloat(float64(value), 'f', -1, 64)
	case int:
		data = strconv.Itoa(value)
	case uint:
		data = strconv.Itoa(int(value))
	case int8:
		data = strconv.Itoa(int(value))
	case uint8:
		data = strconv.Itoa(int(value))
	case int16:
		data = strconv.Itoa(int(value))
	case uint16:
		data = strconv.Itoa(int(value))
	case int32:
		data = strconv.Itoa(int(value))
	case uint32:
		data = strconv.Itoa(int(value))
	case int64:
		data = strconv.FormatInt(value, 10)
	case uint64:
		data = strconv.FormatUint(value, 10)
	case bool:
		if value {
			data = "true"
		} else {
			data = "false"
		}
	case string:
		data = value
	case []byte:
		data = B2S(value)
	default:
		data = fmt.Sprintf("%v", value)
	}
	return data
}
