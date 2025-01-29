package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/fatih/color"
	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"github.com/oschwald/maxminddb-golang/v2"
)

// Add color variables
var (
	successColor = color.New(color.FgGreen).SprintFunc()
	errorColor   = color.New(color.FgRed).SprintFunc()
	warnColor    = color.New(color.FgYellow).SprintFunc()
	infoColor    = color.New(color.FgCyan).SprintFunc()
)

// Add after the color variables
var reservedNetworks = []*net.IPNet{
	mustParseCIDR("10.0.0.0/8"),      // RFC1918 Private IPv4
	mustParseCIDR("172.16.0.0/12"),   // RFC1918 Private IPv4
	mustParseCIDR("192.168.0.0/16"),  // RFC1918 Private IPv4
	mustParseCIDR("fc00::/7"),        // RFC4193 Unique Local IPv6
	mustParseCIDR("fe80::/10"),       // Link Local IPv6
	mustParseCIDR("127.0.0.0/8"),     // Loopback IPv4
	mustParseCIDR("::1/128"),         // Loopback IPv6
	mustParseCIDR("169.254.0.0/16"),  // Link Local IPv4
	mustParseCIDR("0.0.0.0/8"),       // RFC1122 "This host on this network"
	mustParseCIDR("::/128"),          // Unspecified IPv6
	mustParseCIDR("100.64.0.0/10"),   // RFC6598 Shared Address Space
	mustParseCIDR("192.0.0.0/24"),    // RFC5736 IETF Protocol Assignments
	mustParseCIDR("192.0.2.0/24"),    // RFC5737 TEST-NET-1
	mustParseCIDR("198.51.100.0/24"), // RFC5737 TEST-NET-2
	mustParseCIDR("203.0.113.0/24"),  // RFC5737 TEST-NET-3
	mustParseCIDR("224.0.0.0/4"),     // Multicast IPv4
	mustParseCIDR("ff00::/8"),        // Multicast IPv6
}

func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func isReservedNetwork(ip net.IP) bool {
	for _, reserved := range reservedNetworks {
		if reserved.Contains(ip) {
			return true
		}
	}
	return false
}

func main() {
	app := kingpin.New("mmdbmerge", "A tool to merge multiple MMDB files")
	app.HelpFlag.Short('h')
	app.UsageWriter(os.Stdout)

	inputFiles := app.Arg("input", "Input MMDB files (minimum 2)").
		Required().
		ExistingFiles()

	outputFile := app.Flag("output", "Output MMDB file").
		Short('o').
		Default("combined.mmdb").
		String()

	debug := app.Flag("debug", "Enable debug logging").
		Bool()

	// Show help if no args provided
	if len(os.Args) == 1 {
		app.Usage(nil)
		os.Exit(0)
	}

	kingpin.MustParse(app.Parse(os.Args[1:]))

	// Check if there are at least 2 input files
	if len(*inputFiles) < 2 {
		fmt.Println(errorColor("At least 2 input files are required"))
		app.Usage(os.Args[1:])
		os.Exit(1)
	}

	// Display input files and their validation status
	for _, file := range *inputFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			log.Printf("%s File not found: %s", errorColor("ERROR:"), file)
			os.Exit(1)
		} else {
			log.Printf("%s Valid file: %s", successColor("VALID:"), file)
		}
	}

	if *debug {
		log.Printf("%s Processing input files: %v", infoColor("DEBUG:"), *inputFiles)
	}

	// Create new MMDB writer
	writer, err := createWriter(*inputFiles, *debug)
	if err != nil {
		log.Fatal(errorColor(fmt.Sprintf("Error creating writer: %v", err)))
	}

	// Track total networks for final stats
	var totalNetworks, totalSkipped int
	var totalIPs uint64

	// Process each input file
	for _, file := range *inputFiles {
		if *debug {
			log.Printf("%s Processing file: %s", infoColor("DEBUG:"), file)
		}
		stats, err := processFile(writer, file, *debug)
		if err != nil {
			log.Fatal(errorColor(fmt.Sprintf("Error processing %s: %v", file, err)))
		}
		totalNetworks += stats.networks
		totalSkipped += stats.skipped
		totalIPs += stats.ips
	}

	if *debug {
		log.Printf("%s Writing output to: %s", infoColor("DEBUG:"), *outputFile)
	}

	// Write the merged database
	if err := writeDatabase(writer, *outputFile); err != nil {
		log.Fatal(errorColor(fmt.Sprintf("Error writing database: %v", err)))
	}

	// Verify the output file
	outputReader, err := maxminddb.Open(*outputFile)
	if err != nil {
		log.Fatal(errorColor(fmt.Sprintf("Error opening output file: %v", err)))
	}
	defer outputReader.Close()

	var outputNetworks int
	var outputIPs uint64
	for network := range outputReader.Networks() {
		outputNetworks++
		_, ipnet, _ := net.ParseCIDR(network.Prefix().String())
		outputIPs += countIPs(ipnet)
	}

	log.Printf("%s: Total networks: %d (IPs: %d, skipped: %d)",
		successColor("Final Stats"), totalNetworks, totalIPs, totalSkipped)
	log.Printf("%s: %s contains %d networks (IPs: %d)",
		successColor("Output"), *outputFile, outputNetworks, outputIPs)

	// Exit with status 0 on success
	os.Exit(0)
}

func createWriter(files []string, debug bool) (*mmdbwriter.Tree, error) {
	// Get filenames for description
	var fileNames []string
	for _, file := range files {
		name := filepath.Base(strings.TrimSuffix(file, ".mmdb"))
		fileNames = append(fileNames, name)
		if debug {
			log.Printf("%s Added filename to description: %s", infoColor("DEBUG:"), name)
		}
	}

	if debug {
		log.Printf("%s Creating writer with description: Combined %s", infoColor("DEBUG:"), strings.Join(fileNames, ", "))
	}

	return mmdbwriter.New(mmdbwriter.Options{
		DatabaseType: "Combined-DB",
		Description: map[string]string{
			"en": fmt.Sprintf("Combined %s", strings.Join(fileNames, ", ")),
		},
		Languages:  []string{"en"},
		IPVersion:  6,
		RecordSize: 28,
	})
}

type fileStats struct {
	networks int
	skipped  int
	ips      uint64
}

// Add helper function to count IPs in a network
func countIPs(ipnet *net.IPNet) uint64 {
	ones, bits := ipnet.Mask.Size()
	if bits-ones >= 64 {
		return 0 // Avoid overflow for very large networks
	}
	return 1 << uint(bits-ones)
}

func processFile(writer *mmdbwriter.Tree, fullPath string, debug bool) (fileStats, error) {
	reader, err := maxminddb.Open(fullPath)
	if err != nil {
		return fileStats{}, fmt.Errorf("opening MMDB file: %w", err)
	}
	defer reader.Close()

	if debug {
		log.Printf("%s Opened MMDB file: %s", infoColor("DEBUG:"), fullPath)
	}

	baseName := filepath.Base(fullPath)
	source := strings.TrimSuffix(baseName, ".mmdb")
	if debug {
		log.Printf("%s Using source name: %s", infoColor("DEBUG:"), source)
	}

	fromData := mmdbtype.Map{
		mmdbtype.String("from"): mmdbtype.String(source),
	}

	stats := fileStats{}

	for network := range reader.Networks() {
		prefix := network.Prefix()
		if isReservedNetwork(prefix.Addr().AsSlice()) {
			log.Printf("%s Skipping reserved network: %s (from %s)", warnColor("WARN:"), prefix, source)
			stats.skipped++
			continue
		}

		_, ipnet, _ := net.ParseCIDR(prefix.String())
		if err := writer.Insert(ipnet, fromData); err != nil {
			if strings.Contains(err.Error(), "reserved network") {
				stats.skipped++
				continue
			}
			return stats, fmt.Errorf("inserting network %s: %w", prefix, err)
		}
		stats.networks++
		stats.ips += countIPs(ipnet)

		if debug && stats.networks%10000 == 0 {
			log.Printf("%s Processed %d networks from %s", infoColor("DEBUG:"), stats.networks, source)
		}
	}

	log.Printf("%s: %s networks: %d (IPs: %d, skipped: %d)",
		infoColor("Stats"), fullPath, stats.networks, stats.ips, stats.skipped)

	return stats, nil
}

func writeDatabase(writer *mmdbwriter.Tree, filepath string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	_, err = writer.WriteTo(f)
	return err
}
