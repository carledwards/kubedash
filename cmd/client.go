package cmd

import (
	"strings"
)

// ArrayFlags allows for repeated flag values
type ArrayFlags []string

func (i *ArrayFlags) String() string {
	return strings.Join(*i, ",")
}

func (i *ArrayFlags) Set(value string) error {
	if strings.Contains(value, ",") {
		values := strings.Split(value, ",")
		*i = append(*i, values...)
	} else {
		*i = append(*i, value)
	}
	return nil
}
