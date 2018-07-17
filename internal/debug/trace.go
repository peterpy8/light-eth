//+build go1.5

package debug

import (
	"errors"
	"os"
	"runtime/trace"

	"github.com/siotchain/siot/logger"
	"github.com/siotchain/siot/logger/glog"
)

// StartGoTrace turns on tracing, writing to the given file.
func (h *HandlerT) StartGoTrace(file string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.traceW != nil {
		return errors.New("trace already in progress")
	}
	f, err := os.Create(expandHome(file))
	if err != nil {
		return err
	}
	if err := trace.Start(f); err != nil {
		f.Close()
		return err
	}
	h.traceW = f
	h.traceFile = file
	glog.V(logger.Info).Infoln("trace started, writing to", h.traceFile)
	return nil
}

// StopTrace stops an ongoing trace.
func (h *HandlerT) StopGoTrace() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	trace.Stop()
	if h.traceW == nil {
		return errors.New("trace not in progress")
	}
	glog.V(logger.Info).Infoln("done writing trace to", h.traceFile)
	h.traceW.Close()
	h.traceW = nil
	h.traceFile = ""
	return nil
}
