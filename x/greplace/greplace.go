package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	Version = "1.0.0"
	Name    = "greplace"
)

// Config holds command line options
type Config struct {
	Silent   bool
	Verbose  bool
	Help     bool
	Version  bool
	Patterns []PatternPair
	Files    []string
}

type PatternPair struct {
	From string
	To   string
}

// parseArgs parses command line arguments similar to the original C program
func parseArgs(args []string) (*Config, error) {
	config := &Config{
		Patterns: make([]PatternPair, 0),
		Files:    make([]string, 0),
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg[0] == '-' && len(arg) > 1 && arg[1] != '-' {
			// Handle single dash options like -sv
			for j := 1; j < len(arg); j++ {
				switch arg[j] {
				case 's':
					config.Silent = true
				case 'v':
					config.Verbose = true
				case 'V':
					config.Version = true
					return config, nil
				case '?', 'h':
					config.Help = true
					return config, nil
				default:
					return nil, fmt.Errorf("unknown option: -%c", arg[j])
				}
			}
			i++
		} else if arg == "--" {
			// Everything after -- is files
			i++
			for i < len(args) {
				config.Files = append(config.Files, args[i])
				i++
			}
			break
		} else {
			// This should be a from/to pattern pair or file
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				// We have a from/to pair
				config.Patterns = append(config.Patterns, PatternPair{
					From: arg,
					To:   args[i+1],
				})
				i += 2
			} else {
				// This must be a file (or error)
				config.Files = append(config.Files, arg)
				i++
			}
		}
	}

	return config, nil
}

// showHelp displays usage information
func showHelp() {
	fmt.Printf("Usage: %s [options] from to [from to ...] [-- files]\n", Name)
	fmt.Printf("       %s [options] from to [from to ...] < input > output\n", Name)
	fmt.Println()
	fmt.Println("This program replaces strings in files or from stdin to stdout.")
	fmt.Println("It accepts a list of from-string/to-string pairs and replaces")
	fmt.Println("each occurrence of a from-string with the corresponding to-string.")
	fmt.Println()
	fmt.Println("Special characters in from string:")
	fmt.Println("  \\^      Match start of line")
	fmt.Println("  \\$      Match end of line")
	fmt.Println("  \\b      Match word boundary")
	fmt.Println("  \\r, \\t, \\v as in C")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -s      Silent mode")
	fmt.Println("  -v      Verbose mode")
	fmt.Println("  -V      Show version")
	fmt.Println("  -h, -?  Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  %s hello hi world universe file.txt\n", Name)
	fmt.Printf("  %s -v old new *.txt\n", Name)
	fmt.Printf("  echo \"hello world\" | %s hello hi\n", Name)
}

// showVersion displays version information
func showVersion() {
	fmt.Printf("%s version %s\n", Name, Version)
	fmt.Println("Go implementation of the fast string replacement utility")
	fmt.Println()
	fmt.Println("This software comes with ABSOLUTELY NO WARRANTY. This is free software,")
	fmt.Println("and you are welcome to modify and redistribute it under the GPL license")
}

// processStream handles stdin/stdout processing
func processStream(replacer *CompleteReplacer, input io.Reader, output io.Writer, verbose bool) error {
	updated, err := replacer.ReplaceReader(input, output)
	if err != nil {
		return fmt.Errorf("stream processing failed: %v", err)
	}

	if verbose && updated {
		fmt.Fprintf(os.Stderr, "Stream processed with replacements\n")
	}

	return nil
}

// Updated processFile function for memory-efficient file processing
func processFile(replacer *CompleteReplacer, filename string, silent, verbose bool) error {
	// Open input file
	input, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("cannot open %s: %v", filename, err)
	}
	defer input.Close()

	// Get file info for permissions
	info, err := input.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat %s: %v", filename, err)
	}

	// Create temporary file in the same directory
	dir := getDir(filename)
	tempFile, err := os.CreateTemp(dir, ".greplace_temp_*")
	if err != nil {
		return fmt.Errorf("cannot create temp file for %s: %v", filename, err)
	}
	tempName := tempFile.Name()

	// Ensure cleanup on any error
	defer func() {
		tempFile.Close()
		if err != nil {
			os.Remove(tempName)
		}
	}()

	// Use streaming replacement to avoid loading entire file into memory
	updated, err := replacer.ReplaceReader(input, tempFile)
	if err != nil {
		return fmt.Errorf("replacement failed for %s: %v", filename, err)
	}

	// Close temp file before rename
	err = tempFile.Close()
	if err != nil {
		os.Remove(tempName)
		return fmt.Errorf("cannot close temp file for %s: %v", filename, err)
	}

	if updated {
		// Set same permissions on temp file
		err = os.Chmod(tempName, info.Mode())
		if err != nil {
			os.Remove(tempName)
			return fmt.Errorf("cannot set permissions on temp file for %s: %v", filename, err)
		}

		// Atomic replace: rename temp file to original
		err = os.Rename(tempName, filename)
		if err != nil {
			os.Remove(tempName)
			return fmt.Errorf("cannot replace %s: %v", filename, err)
		}

		if !silent {
			fmt.Printf("%s converted\n", filename)
		}
	} else {
		// No changes made, remove temp file
		os.Remove(tempName)

		if verbose && !silent {
			fmt.Printf("%s left unchanged\n", filename)
		}
	}

	return nil
}

// isStdinAvailable checks if there's data available on stdin
func isStdinAvailable() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	// Check if stdin is a pipe or has data
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func main() {
	// Parse command line arguments (skip program name)
	args := os.Args[1:]

	config, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Handle special flags
	if config.Help {
		showHelp()
		return
	}

	if config.Version {
		showVersion()
		return
	}

	// Validate that we have patterns
	if len(config.Patterns) == 0 {
		if len(args) == 0 {
			showHelp()
			return
		}
		fmt.Fprintf(os.Stderr, "Error: No replacement patterns specified\n")
		os.Exit(1)
	}

	// Create and configure the replacer
	replacer := NewCompleteReplacer()

	for _, pattern := range config.Patterns {
		err := replacer.AddPattern(pattern.From, pattern.To)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error adding pattern '%s' -> '%s': %v\n",
				pattern.From, pattern.To, err)
			os.Exit(1)
		}
	}

	// Compile the replacer
	err = replacer.Compile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error compiling patterns: %v\n", err)
		os.Exit(1)
	}

	if config.Verbose && !config.Silent {
		stats := replacer.GetStats()
		fmt.Fprintf(os.Stderr, "Compiled %d patterns into DFA with %d states\n",
			stats["patterns"], stats["dfa_states"])
	}

	// Decide processing mode
	if len(config.Files) == 0 {
		// No files specified - check if stdin has data
		if isStdinAvailable() {
			// Process stdin to stdout
			err = processStream(replacer, os.Stdin, os.Stdout, config.Verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: No input files specified and no data on stdin\n")
			os.Exit(1)
		}
	} else {
		// Process each file
		hasErrors := false
		for _, filename := range config.Files {
			err = processFile(replacer, filename, config.Silent, config.Verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				hasErrors = true
			}
		}

		if hasErrors {
			os.Exit(2)
		}
	}
}

// Helper function to get directory of a file path
func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if os.IsPathSeparator(path[i]) {
			return path[:i]
		}
	}
	return "."
}

// Update your main.go to use this instead:
func processStreamWithLineBasedReplacer(replacer *LineBasedReplacer, input io.Reader, output io.Writer, verbose bool) error {
	// For your test case (no newlines), use the no-newlines version
	updated, err := replacer.ReplaceReaderNoNewlines(input, output)
	if err != nil {
		return fmt.Errorf("stream processing failed: %v", err)
	}

	if verbose && updated {
		fmt.Fprintf(os.Stderr, "Stream processed with replacements\n")
	}

	return nil
}
