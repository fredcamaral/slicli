package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	// Version is set during build
	Version = "dev"

	// BuildDate is set during build
	BuildDate = "unknown"
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "slicli",
	Short: "A CLI tool for creating presentations from markdown",
	Long: `slicli is a command-line tool that transforms markdown files into 
beautiful presentations. It provides live reload, themes, and 
an extensible plugin system - all without requiring any compilation 
steps from users.`,
	Version: Version,
}

func main() {
	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	// Execute root command with context
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Set version template
	rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}
Build Date: ` + BuildDate + `
`)

	// Add global flags
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringP("config", "c", "", "Config file (default: ./slicli/config.toml)")
}
