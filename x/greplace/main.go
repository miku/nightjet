package main

import (
	"fmt"
	"log"
	"strings"
)

func debugInPlace() {
	fmt.Println("=== Debug In-Place Replacement ===")

	// Create the same setup as the failing test
	pp := NewPatternProcessor()
	pp.AddPattern("test", "exam")

	builder := NewDFABuilder(pp)
	dfa, err := builder.BuildDFA()
	if err != nil {
		log.Fatalf("Failed to build DFA: %v", err)
	}

	engine := NewReplacementEngine(dfa, pp)

	// Test the exact same strings from the failing test
	lines := []string{
		"this is a test",
		"another test line",
		"no matches here",
	}

	fmt.Println("Original lines:")
	for i, line := range lines {
		fmt.Printf("  [%d] %q\n", i, line)
	}

	// Test each line individually first
	fmt.Println("\nTesting each line individually:")
	for i, line := range lines {
		result, updated, err := engine.ReplaceString(line)
		if err != nil {
			fmt.Printf("  [%d] Error: %v\n", i, err)
		} else {
			fmt.Printf("  [%d] %q -> %q (updated: %v)\n", i, line, result, updated)
		}
	}

	// Now test ReplaceInPlace
	fmt.Println("\nTesting ReplaceInPlace:")
	updated, err := engine.ReplaceInPlace(lines)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Updated: %v\n", updated)
		fmt.Println("Modified lines:")
		for i, line := range lines {
			fmt.Printf("  [%d] %q\n", i, line)
		}
	}

	// Check if any line contains "exam"
	foundExam := false
	for _, line := range lines {
		if strings.Contains(line, "exam") {
			foundExam = true
			break
		}
	}
	fmt.Printf("Found 'exam' in lines: %v\n", foundExam)
}

func main() {
	debugInPlace()
}
