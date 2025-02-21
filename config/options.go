package config

import (
	"flag"
	"os"
)

// DefaultConfigOptions returns the default configuration options
func DefaultConfigOptions() Options {
	opts := Options{
		ProgramName:   os.Args[0],
		EnvPrefix:     "",
		SkipFlags:     false,
		SkipEnv:       false,
		Args:          os.Args[1:],
		ErrorHandling: flag.ContinueOnError,
		UseBuildInfo:  false,
	}

	return opts
}

func setOptions(options Options) Options {

	// Start with default options, then override with provided values
	opts := DefaultConfigOptions()

	// Only override non-zero values from the provided options
	if options.ProgramName != "" {
		opts.ProgramName = options.ProgramName
	}
	if options.EnvPrefix != "" {
		opts.EnvPrefix = options.EnvPrefix
	}

	if options.SkipFlags {
		opts.SkipFlags = true
	}
	if options.SkipEnv {
		opts.SkipEnv = true
	}
	if options.UseBuildInfo {
		opts.UseBuildInfo = true
	}
	if len(options.Args) > 0 {
		opts.Args = options.Args
	}
	if options.ErrorHandling != 0 {
		opts.ErrorHandling = options.ErrorHandling
	}

	return opts
}
