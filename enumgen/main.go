package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	fmt.Println("args + ", os.Args[1:])
	// We expect to be run via go:generate
	sourceFile := os.Getenv("GOFILE")
	fmt.Println(sourceFile)
	if sourceFile == "" {
		log.Fatal("This tool is meant to be run using go:generate")
	}

	info := commentInfo{}

	// Log what we're doing
	if info.outputPackage != info.sourcePackage {
		fmt.Printf("Generating enums from package %s into package %s\n",
			info.sourcePackage, info.outputPackage)
	} else {
		fmt.Printf("Generating enums in package %s\n", info.sourcePackage)
	}

	// Find all valid maps in the file
	maps, err := findMapsInFile(sourceFile)
	if err != nil {
		log.Fatalf("Error finding maps: %v", err)
	}

	if len(maps) == 0 {
		log.Fatalf("No valid maps found in %s", sourceFile)
	}

	// Process each map
	fmt.Printf("Found %d maps to process:\n", len(maps))

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(sourceFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Error creating output directory: %v", err)
	}

	for _, mapInfo := range maps {
		fmt.Printf("Processing map: %s (key: %s, value: %s)\n",
			mapInfo.Name, mapInfo.KeyType, mapInfo.ValueType)

		// Extract the map data
		mapData, err := extractMapData(mapInfo)
		if err != nil {
			log.Printf("Error extracting data from map %s: %v - skipping",
				mapInfo.Name, err)
			continue
		}

		// Generate the enum code using the specified output package
		code, err := generateEnum(mapData, mapInfo.Name, info.outputPackage)
		if err != nil {
			log.Printf("Error generating enum for map %s: %v - skipping",
				mapInfo.Name, err)
			continue
		}

		// Determine output filename
		outputFile := filepath.Join(outputDir, deriveOutputFileName(mapInfo.Name))

		// Write the generated code
		if err := os.WriteFile(outputFile, code, 0644); err != nil {
			log.Printf("Error writing file for map %s: %v - skipping",
				mapInfo.Name, err)
			continue
		}

		fmt.Printf("Successfully generated %s\n", outputFile)
	}

	fmt.Println("Generation complete!")
}
