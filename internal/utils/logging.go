package utils

import (
	"fmt"
	"log"
)

const (
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Reset  = "\033[0m"
)

// INFO logs an information-level message
func INFO(msg ...any) {
	log.Printf("%s%s\n", string(Green)+StringJustifyL("[INFO]", 10)+string(Reset), fmt.Sprint(msg...))
}

// WARN logs a warning-level message
func WARN(msg ...any) {
	log.Printf("%s%s\n", string(Yellow)+"[WARNING] "+string(Reset), fmt.Sprint(msg...))
}

// ERROR logs an error-level message
func ERROR(msg ...any) {
	log.Printf("%s%s\n", string(Red)+StringJustifyL("[ERROR]", 10)+string(Reset), fmt.Sprint(msg...))
}

func FATAL(msg ...any) {
	log.Fatalf("%s%s\n", string(Red)+StringJustifyL("[FATAL]", 10)+string(Reset), fmt.Sprint(msg...))
}
