package logger

import "errors"

var errNilStore = errors.New("logger: LocalLogStore is required")
