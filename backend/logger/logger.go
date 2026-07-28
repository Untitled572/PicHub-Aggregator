package logger

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxLines = 200

var (
	file  *os.File
	mu    sync.Mutex
	count int
)

func Init(dataDir string) {
	logDir := filepath.Join(dataDir, "logs")
	os.MkdirAll(logDir, 0755)

	var err error
	file, err = os.OpenFile(filepath.Join(logDir, "pichub.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		return
	}
	count = countLines()
}

func System(format string, args ...any) {
	write("SYSTEM", format, args...)
}

func Access(format string, args ...any) {
	write("ACCESS", format, args...)
}

func Error(format string, args ...any) {
	write("ERROR", format, args...)
}

func write(tag, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] [%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), tag, msg)

	fmt.Fprint(os.Stdout, line)

	if file == nil {
		return
	}

	file.WriteString(line)
	count++

	if count > maxLines {
		rotate()
	}
}

func countLines() int {
	if file == nil {
		return 0
	}
	n := 0
	f, err := os.Open(file.Name())
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		n++
	}
	return n
}

func rotate() {
	name := file.Name()
	f, err := os.Open(name)
	if err != nil {
		return
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) <= maxLines {
		return
	}

	lines = lines[len(lines)-maxLines:]

	file.Close()

	nf, err := os.Create(name)
	if err != nil {
		return
	}
	defer nf.Close()

	for _, l := range lines {
		fmt.Fprintln(nf, l)
	}

	file = nf
	count = len(lines)
}

func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		file.Close()
		file = nil
	}
}
