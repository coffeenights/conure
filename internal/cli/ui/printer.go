// Package ui centralizes user-facing output for the conure CLI: colored
// messages, tables, spinners, and the JSON-vs-text output split. Commands
// should print through this package rather than calling fmt.Println directly
// so output style stays consistent and JSON mode is honored uniformly.
package ui

import (
	"fmt"

	"github.com/fatih/color"
)

var (
	successC = color.New(color.FgGreen, color.Bold)
	errorC   = color.New(color.FgRed, color.Bold)
	infoC    = color.New(color.FgCyan)
	headerC  = color.New(color.FgWhite, color.Bold)
	dimC     = color.New(color.FgHiBlack)
)

func Success(format string, a ...any) { successC.Printf(format, a...) }
func SuccessLn(s string)              { successC.Println(s) }

func Error(format string, a ...any) { errorC.Printf(format, a...) }
func ErrorLn(s string)              { errorC.Println(s) }

func Info(format string, a ...any) { infoC.Printf(format, a...) }
func InfoLn(s string)              { infoC.Println(s) }

func Header(format string, a ...any) { headerC.Printf(format, a...) }
func HeaderLn(s string)              { headerC.Println(s) }

func Dim(format string, a ...any) { dimC.Printf(format, a...) }
func DimLn(s string)              { dimC.Println(s) }

// Plain prints uncolored text. Use when a colored printer would be wrong
// (e.g. mixed-content lines) but you still want it to route through ui.
func Plain(format string, a ...any) { fmt.Printf(format, a...) }
func PlainLn(s string)              { fmt.Println(s) }
