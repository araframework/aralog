package aralog

import (
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

// These flags define which text to prefix to each log entry generated by the Logger.
const (
// Bits or'ed together to control what's printed. There is no control over the
// order they appear (the order listed here) or the format they present (as
// described in the comments).  A colon appears after these items:
//	2009/01/23 01:23:23.123123 /a/b/c/d.go:23: message
	Ldate = 1 << iota     // the date: 2009/01/23
	Ltime                         // the time: 01:23:23
	Lmicroseconds                 // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                     // full file name and line number: /a/b/c/d.go:23
	Lshortfile                    // final file name element and line number: d.go:23. overrides Llongfile
	LstdFlags = Ldate | Ltime // initial values for the standard logger
)

// A Logger represents an active logging object that generates lines of
// output to an io.Writer.  Each logging operation makes a single call to
// the Writer's Write method.  A Logger can be used simultaneously from
// multiple goroutines; it guarantees to serialize access to the Writer.
type Logger struct {
	mu      sync.Mutex // ensures atomic writes; protects the following fields
	prefix  string     // prefix to write at beginning of each line
	flag    int        // properties
	out     io.Writer  // destination for output
	buf     []byte     // for accumulating text to write
	size    uint // current size of log file
	path    string // file path if output to a file
	maxsize uint // minimal maxsize should >= 1MB
}

var currentOutFile *os.File

// New creates a new Logger.   The out variable sets the
// destination to which log data will be written.
// The prefix appears at the beginning of each generated log line.
// The flag argument defines the logging properties.
func New(out io.Writer, prefix string, flag int) *Logger {
	return &Logger{out: out, prefix: prefix, flag: flag}
}

// NewFileLogger create a new Logger which output to a file specified
func NewFileLogger(path string, flag int) (*Logger, error) {
	return NewRollFileLogger(path, 1024*1024*10, flag)
}

// NewRollFileLogger create a new Logger which output to a file specified path,
// and roll at specified size
func NewRollFileLogger(path string, maxsize uint, flag int) (*Logger, error) {
	out, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}

	currentOutFile = out

	// minimal maxsize should >= 1MB
	if maxsize < 1024 * 1024 {
		maxsize = 1024 * 1024 * 10
	}

	return &Logger{out: out, prefix: "", flag: flag, path: path, maxsize: maxsize}, nil
}

//var std = New(os.Stderr, "", LstdFlags)

// Cheap integer to fixed-width decimal ASCII.  Give a negative width to avoid zero-padding.
// Knows the buffer has capacity.
func itoa(buf *[]byte, i int, wid int) {
	var u uint = uint(i)
	if u == 0 && wid <= 1 {
		*buf = append(*buf, '0')
		return
	}

	// Assemble decimal in reverse order.
	var b [32]byte
	bp := len(b)
	for ; u > 0 || wid > 0; u /= 10 {
		bp--
		wid--
		b[bp] = byte(u%10) + '0'
	}
	*buf = append(*buf, b[bp:]...)
}

func (l *Logger) formatHeader(buf *[]byte, t time.Time, file string, line int) {
	*buf = append(*buf, l.prefix...)
	if l.flag&(Ldate|Ltime|Lmicroseconds) != 0 {
		if l.flag&Ldate != 0 {
			year, month, day := t.Date()
			itoa(buf, year, 4)
			*buf = append(*buf, '/')
			itoa(buf, int(month), 2)
			*buf = append(*buf, '/')
			itoa(buf, day, 2)
			*buf = append(*buf, ' ')
		}
		if l.flag&(Ltime|Lmicroseconds) != 0 {
			hour, min, sec := t.Clock()
			itoa(buf, hour, 2)
			*buf = append(*buf, ':')
			itoa(buf, min, 2)
			*buf = append(*buf, ':')
			itoa(buf, sec, 2)
			if l.flag&Lmicroseconds != 0 {
				*buf = append(*buf, '.')
				itoa(buf, t.Nanosecond()/1e3, 6)
			}
			*buf = append(*buf, ' ')
		}
	}
	if l.flag&(Lshortfile|Llongfile) != 0 {
		if l.flag&Lshortfile != 0 {
			short := file
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' {
					short = file[i+1:]
					break
				}
			}
			file = short
		}
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, line, -1)
		*buf = append(*buf, ": "...)
	}
}

// Output writes the output for a logging event.  The string s contains
// the text to print after the prefix specified by the flags of the
// Logger.  A newline is appended if the last character of s is not
// already a newline.  Calldepth is used to recover the PC and is
// provided for generality, although at the moment on all pre-defined
// paths it will be 2.
func (l *Logger) output(calldepth int, s string) error {
	now := time.Now() // get this early.
	var file string
	var line int
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.flag&(Lshortfile|Llongfile) != 0 {
		// release lock while getting caller info - it's expensive.
		l.mu.Unlock()
		var ok bool
		_, file, line, ok = runtime.Caller(calldepth)
		if !ok {
			file = "???"
			line = 0
		}
		l.mu.Lock()
	}
	l.buf = l.buf[:0]
	l.formatHeader(&l.buf, now, file, line)
	l.buf = append(l.buf, s...)
	if len(s) > 0 && s[len(s)-1] != '\n' {
		l.buf = append(l.buf, '\n')
	}

	if len(l.path) > 0 {
		err := l.rollFile(now)
		if err != nil {
			return err
		}
	}
	_, err := l.out.Write(l.buf)
	return err
}

func (l *Logger) rollFile(now time.Time) error {
	l.size += uint(len(l.buf))
	// file rotation if size > maxsize
	if l.size > l.maxsize {

		// close file before rename it
		if currentOutFile != nil {
			// ignore if Close() failed
			err := currentOutFile.Close()
			if err != nil {
				l.buf = append(l.buf, "[XXX] ARALOGGER ERROR: Close current output file failed, " + err.Error(), '\n')
			}
		}

		newPath := l.path
		err := os.Rename(l.path,
			l.path + string(now.Year()) + string(now.Month()) + string(now.Day()) +
			string(now.Hour()) + string(now.Minute()) + string(now.Second()))
		if err != nil {
			l.buf = append(l.buf, "[XXX] ARALOGGER ERROR: Rolling file failed, " + err.Error(), '\n')
			newPath = l.path + string(now.Unix())
		}

		newOut, err := os.OpenFile(newPath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}

		currentOutFile = newOut
		l.out = newOut
		l.size = uint(len(l.buf))
	}

	return nil
}

func (l *Logger) Debug(s string) error {
	err := l.output(2, s)
	return err
}
