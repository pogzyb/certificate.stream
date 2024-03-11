package main

import (
	"certificate.stream/service/cmd"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

func main() { cmd.Execute() }
