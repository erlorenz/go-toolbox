package cfgx

import (
	"flag"
	"os"
)

// DefaultConfigOptions are the default set of configuration options.
// Each option can be overridden.
var DefaultConfigOptions = Options{
	ProgramName:   os.Args[0],
	EnvPrefix:     "",
	SkipFlags:     false,
	SkipEnv:       false,
	Args:          os.Args[1:],
	ErrorHandling: flag.ContinueOnError,
	UseBuildInfo:  true,
	Sources:       []Source{},
}

func setOptions(options Options) Options {

	// Start with default options, then override with provided values
	opts := DefaultConfigOptions

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
	if options.Args != nil {
		opts.Args = options.Args
	}
	if options.ErrorHandling != flag.ContinueOnError {
		opts.ErrorHandling = options.ErrorHandling
	}

	return opts
}
