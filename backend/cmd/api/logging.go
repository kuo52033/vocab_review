package main

import (
	"bytes"
	"io"
	"log/slog"
	"strconv"
)

func newLogger(out io.Writer, color string) *slog.Logger {
	logOutput := logFormatWriter{
		out:   out,
		color: logColorEnabled(color),
	}
	return slog.New(slog.NewTextHandler(logOutput, &slog.HandlerOptions{
		ReplaceAttr: replaceLogAttr,
	}))
}

type logFormatWriter struct {
	out   io.Writer
	color bool
}

func (w logFormatWriter) Write(data []byte) (int, error) {
	lines := bytes.SplitAfter(data, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		formatted := stripTimeKey(line)
		if w.color {
			if colored, ok := colorStatusValue(formatted); ok {
				formatted = colored
			}
		}
		if _, err := w.out.Write(formatted); err != nil {
			return 0, err
		}
	}
	return len(data), nil
}

func stripTimeKey(line []byte) []byte {
	const prefix = "time="
	if !bytes.HasPrefix(line, []byte(prefix)) {
		return line
	}

	value, rest, ok := bytes.Cut(line[len(prefix):], []byte(" "))
	if !ok {
		return line
	}

	value = bytes.Trim(value, `"`)
	return bytes.Join([][]byte{value, rest}, []byte(" "))
}

func colorStatusValue(line []byte) ([]byte, bool) {
	const key = "status="
	index := bytes.Index(line, []byte(key))
	if index == -1 {
		return nil, false
	}
	valueStart := index + len(key)
	valueEnd := valueStart
	for valueEnd < len(line) && line[valueEnd] >= '0' && line[valueEnd] <= '9' {
		valueEnd++
	}
	if valueStart == valueEnd {
		return nil, false
	}

	status, err := strconv.Atoi(string(line[valueStart:valueEnd]))
	if err != nil {
		return nil, false
	}

	color := statusColor(status)
	if color == "" {
		return nil, false
	}

	colored := make([]byte, 0, len(line)+len(color)+len("\033[0m"))
	colored = append(colored, line[:valueStart]...)
	colored = append(colored, color...)
	colored = append(colored, line[valueStart:valueEnd]...)
	colored = append(colored, "\033[0m"...)
	colored = append(colored, line[valueEnd:]...)
	return colored, true
}

func statusColor(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "\033[32m"
	case status >= 300 && status < 400:
		return "\033[36m"
	case status >= 400 && status < 500:
		return "\033[33m"
	case status >= 500:
		return "\033[31m"
	default:
		return ""
	}
}

func replaceLogAttr(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.TimeKey {
		return slog.String(slog.TimeKey, "["+attr.Value.Time().Format("2006-01-02T15:04:05.000")+"]")
	}
	return attr
}
