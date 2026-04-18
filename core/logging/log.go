package logging

import (
	"io"
	"log"
	"os"
	"sync"
)

type Logger = log.Logger

// glob is the default logger used by the package-level functions.
var glob *Logger

func init() {
	sync.OnceFunc(func() {
		glob = log.New(os.Stdout, "", log.LstdFlags)
	})()
}

// New creates a new Logger.
func New(out io.Writer, prefix string, flag int) *Logger {
	return log.New(out, prefix, flag)
}

// DefaultLogger returns the default logger used by the package-level functions.
func DefaultLogger() *Logger {
	return glob
}

// Print calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Print.
func Print(v ...any) {
	glob.Print(v...)
}

// Printf calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Printf.
func Printf(format string, v ...any) {
	glob.Printf(format, v...)
}

// Println calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Println.
func Println(v ...any) {
	glob.Println(v...)
}

// Fatal is equivalent to glob.Print() followed by a call to os.Exit(1).
func Fatal(v ...any) {
	glob.Fatal(v...)
}

// Fatalf is equivalent to glob.Printf() followed by a call to os.Exit(1).
func Fatalf(format string, v ...any) {
	glob.Fatalf(format, v...)
}

// Fatalln is equivalent to glob.Println() followed by a call to os.Exit(1).
func Fatalln(v ...any) {
	glob.Fatalln(v...)
}

// Panic is equivalent to glob.Print() followed by a call to panic().
func Panic(v ...any) {
	glob.Panic(v...)
}

// Panicf is equivalent to glob.Printf() followed by a call to panic().
func Panicf(format string, v ...any) {
	glob.Panicf(format, v...)
}

// Panicln is equivalent to glob.Println() followed by a call to panic().
func Panicln(v ...any) {
	glob.Panicln(v...)
}

// Flags returns the output flags for the logger.
func Flags() int {
	return glob.Flags()
}

// SetFlags sets the output flags for the logger.
func SetFlags(flag int) {
	glob.SetFlags(flag)
}

// Prefix returns the output prefix for the logger.
func Prefix() string {
	return glob.Prefix()
}

// SetPrefix sets the output prefix for the logger.
func SetPrefix(prefix string) {
	glob.SetPrefix(prefix)
}

// SetOutput sets the output destination for the standard logger.
func SetOutput(w io.Writer) {
	glob.SetOutput(w)
}

// Writer returns the output destination for the logger.
func Writer() io.Writer {
	return glob.Writer()
}

// Output writes the output for a logging event.
func Output(calldepth int, s string) error {
	return glob.Output(calldepth, s)
}
