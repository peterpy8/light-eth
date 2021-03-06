package errs

import (
	"fmt"

	"github.com/siotchain/siot/logger"
	"github.com/siotchain/siot/logger/glog"
)

/*
Errors implements an error handler providing standardised errors for a package.
Fields:

 Errors:
  a map from error codes to description

 Package:
  name of the package/component

 Level:
  a function mapping error code to logger.LogLevel (severity)
  if not given, errors default to logger.InfoLevel
*/
type Errors struct {
	Errors  map[int]string
	Package string
	Level   func(code int) logger.LogLevel
}

/*
Error implements the standard go error interface.

  errors.New(code, format, configure ...interface{})

Prints as:

 [package] description: details

where details is fmt.Sprintf(self.format, self.configure...)
*/
type Error struct {
	Code    int
	Name    string
	Package string
	level   logger.LogLevel
	message string
	format  string
	params  []interface{}
}

func (self *Errors) New(code int, format string, params ...interface{}) *Error {
	name, ok := self.Errors[code]
	if !ok {
		panic("invalid error code")
	}
	level := logger.InfoLevel
	if self.Level != nil {
		level = self.Level(code)
	}
	return &Error{
		Code:    code,
		Name:    name,
		Package: self.Package,
		level:   level,
		format:  format,
		params:  params,
	}
}

func (self Error) Error() (message string) {
	if len(message) == 0 {
		self.message = fmt.Sprintf("[%s] ERROR: %s", self.Package, self.Name)
		if self.format != "" {
			self.message += ": " + fmt.Sprintf(self.format, self.params...)
		}
	}
	return self.message
}

func (self Error) Log(v glog.Verbose) {
	if v {
		v.Infoln(self)
	}
}

/*
err.Fatal() is true if err's severity level is 0 or 1 (logger.ErrorLevel or logger.Silence)
*/
func (self *Error) Fatal() (fatal bool) {
	if self.level < logger.WarnLevel {
		fatal = true
	}
	return
}
