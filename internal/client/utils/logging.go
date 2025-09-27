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
	log.Println(string(Green), "[INFO]", string(Reset), fmt.Sprint(msg...))
}

// WARN logs a warning-level message
func WARN(msg ...any) {
	log.Println(string(Yellow), "[WARNING]", string(Reset), fmt.Sprint(msg...))
}

// ERROR logs an error-level message
func ERROR(msg ...any) {
	log.Println(string(Red), "[ERROR]", string(Reset), fmt.Sprint(msg...))
}

func FATAL(msg ...any) {
	log.Fatalln(string(Red), "[FATAL]", string(Reset), fmt.Sprint(msg...))
}
