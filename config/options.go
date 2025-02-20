package config

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

	if len(options.Args) > 0 {
		opts.Args = options.Args
	}
	if options.ErrorHandling != 0 {
		opts.ErrorHandling = options.ErrorHandling
	}

	return opts
}
